package service

import "github.com/energieip/common-sensor-go/pkg/driversensor"

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
