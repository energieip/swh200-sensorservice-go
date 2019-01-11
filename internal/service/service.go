package service

import (
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

//Initialize service
func (s *SensorService) Initialize(confFile string) error {
	s.sensors = make(map[string]*driversensor.Sensor)
	hostname, _ := os.Hostname()
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
	err = s.db.CreateDB(driversensor.DbStatus)
	if err != nil {
		rlog.Warn("Create DB ", err.Error())
	}
	err = s.db.CreateTable(driversensor.DbStatus, driversensor.TableName, &driversensor.Sensor{})
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
		IP:               conf.LocalBroker.IP,
		Port:             conf.LocalBroker.Port,
		ClientName:       clientID,
		Callbacks:        callbacks,
		LogLevel:         conf.LogLevel,
		User:             conf.LocalBroker.Login,
		Password:         conf.LocalBroker.Password,
		ClientKey:        conf.LocalBroker.KeyPath,
		ServerCertificat: conf.LocalBroker.CaPath,
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
