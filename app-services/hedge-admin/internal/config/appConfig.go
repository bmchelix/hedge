/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"strconv"
)

type AppConfig struct {
	ContentDir                  string
	NodeType                    string
	NodeId                      string
	NodeHostName                string
	ConsulURL                   string
	MqttURL                     string
	DefaultNodeGroupName        string
	DefaultNodeGroupDisplayName string
	MaxImportExportFileSizeMB   int
}

const MaxImportExportFileSizeDefaultMB = 5

func NewAppConfig() *AppConfig {
	appConfig := new(AppConfig)
	return appConfig
}

func (appConfig *AppConfig) LoadAppConfigurations(service interfaces.ApplicationService) {
	contentDir, err := service.GetAppSetting("ContentDir")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	nodeType, err := service.GetAppSetting("NodeType")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	nodeHostName, err := service.GetAppSetting("Node_HostName")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	nodeId, err := service.GetAppSetting("Node_Id")
	if err != nil {
		service.LoggingClient().Error(err.Error())
		nodeId = nodeHostName
	}

	consulHost, err := service.GetAppSetting("ConsulHost")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	consulPort, err := service.GetAppSetting("ConsulPort")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}

	mqttURL, err := service.GetAppSetting("MqttURL")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}

	defaultNodeGroupName, err := service.GetAppSetting("DefaultNodeGroup_Name")
	if err != nil {
		service.LoggingClient().Errorf("Error reading DefaultNodeGroup_Name: %v", err)
		appConfig.DefaultNodeGroupName = "DefaultGroupName"
	} else {
		appConfig.DefaultNodeGroupName = defaultNodeGroupName
	}
	defaultNodeGroupDisplayName, err := service.GetAppSetting("DefaultNodeGroup_Display_Name")
	if err != nil {
		service.LoggingClient().Errorf("Error reading DefaultNodeGroup_Display_Name: %v", err)
		appConfig.DefaultNodeGroupDisplayName = "Nodes"
	} else {
		appConfig.DefaultNodeGroupDisplayName = defaultNodeGroupDisplayName
	}

	var maxImportExportFileSizeMB int
	maxImportExportFileSizeMBStr, err := service.GetAppSetting("MaxImportExportFileSizeMB")
	if err != nil {
		service.LoggingClient().Error(err.Error())
		maxImportExportFileSizeMB = MaxImportExportFileSizeDefaultMB
	}
	maxImportExportFileSizeMB, err = strconv.Atoi(maxImportExportFileSizeMBStr)
	if err != nil {
		service.LoggingClient().Error(err.Error())
		maxImportExportFileSizeMB = MaxImportExportFileSizeDefaultMB
	}

	service.LoggingClient().Infof("ContentDir DB URL: %v, NodeName: %v, Consul: %s:%s", contentDir, nodeHostName, consulHost, consulPort)

	appConfig.ContentDir = contentDir
	appConfig.NodeType = nodeType
	appConfig.NodeId = nodeId
	appConfig.NodeHostName = nodeHostName
	appConfig.ConsulURL = consulHost + ":" + consulPort
	appConfig.MqttURL = mqttURL
	appConfig.MaxImportExportFileSizeMB = maxImportExportFileSizeMB
}
