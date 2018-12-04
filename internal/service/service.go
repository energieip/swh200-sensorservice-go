package service

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/energieip/common-database-go/pkg/database"
	"github.com/energieip/common-network-go/pkg/network"
	"github.com/energieip/common-sensor-go/pkg/driversensor"
	pkg "github.com/energieip/common-service-go/pkg/service"
	"github.com/energieip/common-tools-go/pkg/tools"
	"github.com/romana/rlog"
)

//SensorService content
type SensorService struct {
	db      database.DatabaseInterface
	broker  network.NetworkInterface //Local Broker for drivers communication
	sensors map[string]*driversensor.Sensor
	mac     string //Switch mac address
}

func (s *SensorService) updateDatabase(sensor driversensor.Sensor) error {
	var dbID string
	if val, ok := s.sensors[sensor.Mac]; ok {
		sensor.ID = val.ID
		dbID = val.ID
		if *val == sensor {
			// No change to register
			return nil
		}
	}

	s.sensors[sensor.Mac] = &sensor
	if dbID != "" {
		// Check if the serial already exist in database (case restart process)
		criteria := make(map[string]interface{})
		criteria["Mac"] = sensor.Mac
		criteria["SwitchMac"] = s.mac
		sensorStored, err := s.db.GetRecord(driversensor.DbName, driversensor.TableName, criteria)
		if err == nil && sensorStored != nil {
			m := sensorStored.(map[string]interface{})
			id, ok := m["id"]
			if !ok {
				id, ok = m["ID"]
			}
			if ok {
				dbID = id.(string)
			}
		}
	}
	var err error

	if dbID == "" {
		dbID, err = s.db.InsertRecord(driversensor.DbName, driversensor.TableName, s.sensors[sensor.Mac])
	} else {
		err = s.db.UpdateRecord(driversensor.DbName, driversensor.TableName, dbID, s.sensors[sensor.Mac])
	}
	if err != nil {
		return err
	}
	s.sensors[sensor.Mac].ID = dbID
	return nil
}

func (s *SensorService) getSensor(mac string) *driversensor.Sensor {
	if val, ok := s.sensors[mac]; ok {
		return val
	}
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	criteria["SwitchMac"] = s.mac
	sensorStored, err := s.db.GetRecord(driversensor.DbName, driversensor.TableName, criteria)
	if err != nil || sensorStored == nil {
		return nil
	}
	cell, _ := driversensor.ToSensor(sensorStored)
	return cell
}

func (s *SensorService) getTopic(mac string) string {
	cell := s.getSensor(mac)
	if cell != nil {
		return cell.Topic
	}
	return ""
}

func (s *SensorService) onSetup(client network.Client, msg network.Message) {
	rlog.Debug("Sensor service onSetup: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var sensor driversensor.SensorSetup
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	topic := s.getTopic(sensor.Mac)
	if topic == "" {
		rlog.Warnf("Sensor %v not found", sensor.Mac)
		return
	}
	url := "/write/" + topic + "/" + driversensor.UrlSetup
	dump, _ := sensor.ToJSON()
	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration for driver " + sensor.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + sensor.Mac + " on topic: " + url + " dump: " + dump)
	}
}

func (s *SensorService) onUpdate(client network.Client, msg network.Message) {
	rlog.Debug("Sensor service onUpdate: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var conf driversensor.SensorConf
	err := json.Unmarshal(msg.Payload(), &conf)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	topic := s.getTopic(conf.Mac)
	if topic == "" {
		rlog.Warnf("Sensor %v not found", conf.Mac)
		return
	}
	url := "/write/" + topic + "/update/settings"
	dump, _ := conf.ToJSON()
	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration to driver " + conf.Mac + " err " + err.Error())
	} else {
		rlog.Info("New update has been sent to " + conf.Mac + " on topic: " + url)
	}
}

func (s *SensorService) onDriverHello(client network.Client, msg network.Message) {
	rlog.Debug("Sensor service Hello: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var sensor driversensor.Sensor
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	sensor.IsConfigured = false
	sensor.Protocol = "MQTT"
	sensor.SwitchMac = s.mac
	err = s.updateDatabase(sensor)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	rlog.Infof("New Sensor driver %v stored on database ", sensor.Mac)
}

func (s *SensorService) onDriverStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug("Sensor service status: Received topic: " + topic + " payload: " + string(msg.Payload()))
	var sensor driversensor.Sensor
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	sensor.SwitchMac = s.mac
	sensor.Protocol = "MQTT"
	topics := strings.Split(topic, "/")
	sensor.Topic = topics[2] + "/" + topics[3]
	err = s.updateDatabase(sensor)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
}

//Initialize service
func (s *SensorService) Initialize(confFile string) error {
	s.sensors = make(map[string]*driversensor.Sensor)
	hostname, err := os.Hostname()
	if err != nil {
		rlog.Error("Cannot read hostname " + err.Error())
		return err
	}
	clientID := "Sensor" + hostname
	s.mac = strings.ToUpper(strings.Replace(tools.GetMac(), ":", "", -1))

	conf, err := pkg.ReadServiceConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}
	os.Setenv("RLOG_LOG_LEVEL", conf.LogLevel)
	os.Setenv("RLOG_LOG_NOTIME", "yes")
	rlog.UpdateEnv()
	rlog.Info("Starting Sensor service")

	db, err := database.NewDatabase(database.RETHINKDB)
	if err != nil {
		rlog.Error("database err " + err.Error())
		return err
	}

	confDb := database.DatabaseConfig{
		IP:   conf.DB.ClientIP,
		Port: conf.DB.ClientPort,
	}
	err = db.Initialize(confDb)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return err
	}
	s.db = db
	err = s.db.CreateDB(driversensor.DbName)
	if err != nil {
		rlog.Warn("Create DB ", err.Error())
	}
	err = s.db.CreateTable(driversensor.DbName, driversensor.TableName, &driversensor.Sensor{})
	if err != nil {
		rlog.Warn("Create table ", err.Error())
	}

	driversBroker, err := network.NewNetwork(network.MQTT)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}
	s.broker = driversBroker

	callbacks := make(map[string]func(client network.Client, msg network.Message))
	callbacks["/read/sensor/+/"+driversensor.UrlHello] = s.onDriverHello
	callbacks["/read/sensor/+/"+driversensor.UrlStatus] = s.onDriverStatus
	callbacks["/write/switch/sensor/setup/config"] = s.onSetup
	callbacks["/write/switch/sensor/update/settings"] = s.onUpdate

	confDrivers := network.NetworkConfig{
		IP:         conf.LocalBroker.IP,
		Port:       conf.LocalBroker.Port,
		ClientName: clientID,
		Callbacks:  callbacks,
		LogLevel:   conf.LogLevel,
	}
	err = s.broker.Initialize(confDrivers)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}

	rlog.Info(clientID + " connected to drivers broker " + conf.LocalBroker.IP)
	rlog.Info("Sensor service started")
	return nil
}

//Stop service
func (s *SensorService) Stop() {
	rlog.Info("Stopping Sensor service")
	s.broker.Disconnect()
	s.db.Close()
	rlog.Info("Sensor service stopped")
}

//Run service mainloop
func (s *SensorService) Run() error {
	select {}
}
