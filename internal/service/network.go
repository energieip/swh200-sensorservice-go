package service

import (
	"encoding/json"
	"strings"

	"github.com/energieip/common-network-go/pkg/network"
	"github.com/energieip/common-sensor-go/pkg/driversensor"
	"github.com/romana/rlog"
)

type SetupCmd struct {
	driversensor.SensorSetup
	CmdType string `json:"cmdType"`
}

// ToJSON dump SetupCmd struct
func (sensor SetupCmd) ToJSON() (string, error) {
	inrec, err := json.Marshal(sensor)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
}

type UpdateCmd struct {
	driversensor.SensorConf
	CmdType string `json:"cmdType"`
}

// ToJSON dump UpdateCmd struct
func (sensor UpdateCmd) ToJSON() (string, error) {
	inrec, err := json.Marshal(sensor)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
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

	setupCmd := SetupCmd{}
	setupCmd.SensorSetup = sensor
	setupCmd.CmdType = "setup"

	dump, _ := setupCmd.ToJSON()
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

	updateCmd := UpdateCmd{}
	updateCmd.SensorConf = conf
	updateCmd.CmdType = "update"

	dump, _ := updateCmd.ToJSON()
	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration to driver " + conf.Mac + " err " + err.Error())
	} else {
		rlog.Info("New update has been sent to " + conf.Mac + " on topic: " + url + " dump: " + dump)
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
