/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/lithammer/shortuuid/v3"
	"os"
	"strconv"
	"strings"
)

const (
	defaultTopicPrefix = "hedge"
)

func GenerateClientId(topic string, clientId string) string {
	return clientId + "-" + shortuuid.New()
}

func BuildMQTTSecretConfig(service interfaces.ApplicationService, topic string, clientId string) (transforms.MQTTSecretConfig, error) {
	// 2) shows how to access the application's specific configuration settings.

	scheme, err := service.GetAppSetting("scheme")
	if err != nil {
		scheme = "tcp"
	}
	mqttServer, err := service.GetAppSetting("MqttServer")
	if err != nil {
		mqttServer = "edgex-mqtt-broker"
	}
	service.LoggingClient().Infof("MQTT Server is %v", mqttServer)

	mqttPort, err := service.GetAppSetting("MqttPort")
	if err != nil {
		mqttPort = "1883"
	}
	port, _ := strconv.ParseInt(mqttPort, 10, 64)
	service.LoggingClient().Infof("MQTT Port is %d", port)

	mqttAuthMode, err := service.GetAppSetting("MqttAuthMode")
	if err != nil {
		mqttAuthMode = "none"
	}
	service.LoggingClient().Infof("MQTT AuthMode is %v", mqttAuthMode)

	mqttSecretName, err := service.GetAppSetting("MqttSecretName")
	if err != nil {
		// This is the path in vault for the calling service
		mqttSecretName = "mbconnection"
	}
	service.LoggingClient().Infof("MQTT SecretPath is %v", mqttSecretName)

	brokerAddress := scheme + "://" + mqttServer + ":" + mqttPort
	// Add the BaseTopicPrefix to topic name so it is in line with what we subscribe to

	publishTopicWithPrefix := BuildTopicNameFromBaseTopicPrefix(topic, "/")
	mqttConfig := transforms.MQTTSecretConfig{
		BrokerAddress:  brokerAddress,
		ClientId:       GenerateClientId(topic, clientId),
		SecretName:     mqttSecretName,
		AutoReconnect:  true,
		KeepAlive:      "30s",
		ConnectTimeout: "60s",
		Topic:          publishTopicWithPrefix,
		QoS:            GetMQTTQoS(service),
		Retain:         false,
		SkipCertVerify: true,
		AuthMode:       mqttAuthMode, //Can be usernamepassword or none
	}
	return mqttConfig, nil
}

func BuildTopicNameFromBaseTopicPrefix(topic string, separator string) string {

	prefix := os.Getenv("MESSAGEBUS_BASETOPICPREFIX")
	if prefix == "" {
		prefix = defaultTopicPrefix
	}
	if !strings.HasPrefix(topic, prefix) {
		return prefix + separator + topic
	}
	return topic
}
