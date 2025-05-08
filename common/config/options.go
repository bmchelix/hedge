/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"strconv"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

// This is for MQTT persistOnError as well as for HTTP request
func GetPersistOnError(service interfaces.ApplicationService) bool {
	lc := service.LoggingClient()
	var SAFEnabled = false
	persistOnError, err := service.GetAppSetting("PersistOnError")
	if err != nil {
		lc.Errorf("PersistOnError parameter not found in the config")
		SAFEnabled = false
	} else {
		SAFEnabled, err = strconv.ParseBool(persistOnError)
		if err != nil {
			lc.Errorf("Invalid value specified for PersistOnError in configuration: %s", err.Error())
		}
	}
	return SAFEnabled
}

func GetMQTTRetain(service interfaces.ApplicationService) bool {
	lc := service.LoggingClient()
	var retained = false
	persistOnError, err := service.GetAppSetting("Retain")
	if err != nil {
		lc.Errorf("PersistOnError parameter not found in the config")
		retained = false
	}
	retained, err = strconv.ParseBool(persistOnError)
	if err != nil {
		lc.Errorf("Invalid value specified for PersistOnError in configuration: %s", err.Error())
	}
	return retained
}

func GetMQTTQoS(service interfaces.ApplicationService) byte {
	lc := service.LoggingClient()
	qoS, err := service.GetAppSetting("QoS")
	if err != nil {
		lc.Errorf("failed to retrieve MqttQoS from configuration: %s", err.Error())
		lc.Info("Set MqttQoS to 0")
		qoS = "0"
	}
	var mqttQoS byte
	if qoS == "1" {
		mqttQoS = 1
	} else if qoS == "2" {
		mqttQoS = 2
	} else {
		mqttQoS = 0
		lc.Debugf("MqttQoS configuration defaulting to 0")
	}
	return mqttQoS
}
