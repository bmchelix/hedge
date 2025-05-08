/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package broker

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type MLBrokerConfig struct {
	LocalMLModelBaseDir string
	MetaDataServiceUrl  string
	EventsPublisherURL  string
	ReadMessageBus      string
}

func NewMLBrokerConfig() *MLBrokerConfig {
	mlMgmtConfig := new(MLBrokerConfig)
	return mlMgmtConfig
}

func (mlCfg *MLBrokerConfig) LoadConfigurations(service interfaces.ApplicationService) {

	lc := service.LoggingClient()

	metaDataServiceUrl, err := service.GetAppSetting("MetaDataServiceUrl")
	if err != nil {
		lc.Error("Error reading the configuration for MetaDataServiceUrl: ", err.Error())
	} else {
		mlCfg.MetaDataServiceUrl = metaDataServiceUrl
	}

	eventPublisherUrl, err := service.GetAppSetting("EventPipelineTriggerURL")
	if err != nil {
		lc.Error("Error reading the configuration for EventPipelineTriggerURL: ", err.Error())
	} else {
		mlCfg.EventsPublisherURL = eventPublisherUrl
	}

	modelBaseDir, err := service.GetAppSetting("LocalMLModelDir")
	if err != nil {
		lc.Error("Error reading the configuration for LocalMLModelDir: ", err.Error())
	} else {
		mlCfg.LocalMLModelBaseDir = modelBaseDir
	}

	readMsgBus, err := service.GetAppSetting("ReadMessageBus")
	if err != nil {
		lc.Error("Error reading the configuration for ReadMessageBus: ", err.Error())
	} else {
		mlCfg.ReadMessageBus = readMsgBus
	}

}
