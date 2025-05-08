/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"strconv"
)

type DataConfig struct {
	AbnormalEventSearchUrl       string  `json:"abnormalEventSearchUrl"` //"https://bmc-iot-adeintqa1-trial.qa.sps.secops.bmc.com/tsws/monitoring/api/v1.0/events/search"
	DataCollectionIntervalInSecs float64 `json:"dataCollectionIntervalInSecs"`
	ApiKey                       string  `json:"apiKey"`
	AbnormalEventSearchAPIKey    string  `json:"abnormalEventSearchAPIKey"`
	EdgeNodeEventTriggerUrl      string  `json:"edgeNodeEventTriggerUrl"`

	// Add hyperparameters as interface, Different algo will have different hyperparameters
	//samplingIntervalInMillis float64
	//TrainingDataDurationInMinutes float64 `json:"trainingDataDurationInMinutes"`
}

func NewDataConfig(edgexSDK interfaces.ApplicationService) (*DataConfig, error) {
	dataConfig := new(DataConfig)

	abnormalEventQueryUrl, err := edgexSDK.GetAppSettingStrings("AbnormalEventSearchUrl")
	if err != nil {
		return nil, fmt.Errorf("AbnormalEventSearchUrl for training not configured")
	}
	dataConfig.AbnormalEventSearchUrl = abnormalEventQueryUrl[0]

	dataCollectionIntervalSecs, err := edgexSDK.GetAppSettingStrings("DataCollectionIntervalInSecs")
	if err != nil {
		return nil, fmt.Errorf("DataCollectionIntervalInSecs for training not configured")
	}
	dataConfig.DataCollectionIntervalInSecs, _ = strconv.ParseFloat(dataCollectionIntervalSecs[0], 64)

	apiKey, err := edgexSDK.GetAppSettingStrings("ApiKey")
	if err != nil {
		return nil, fmt.Errorf("AbnormalEventSearchAPIKey for training not configured")
	}
	dataConfig.ApiKey = apiKey[0]
	dataConfig.AbnormalEventSearchAPIKey = "apiKey " + apiKey[0]

	edgeNodeTriggerUrl, err := edgexSDK.GetAppSettingStrings("EdgeNodeEventTriggerUrl")
	if err != nil {
		return nil, fmt.Errorf("EdgeNodeEventTriggerUrl not configured, this should be for event pipeline on edge to trigger")
	}
	dataConfig.EdgeNodeEventTriggerUrl = edgeNodeTriggerUrl[0]

	return dataConfig, nil
}
