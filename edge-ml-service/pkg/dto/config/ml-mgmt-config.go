/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

// MLMgmtConfig: Configuration for ML Management
type MLMgmtConfig struct {
	// TrainingJobServiceConfigByAlgo is specific to training providers, so should have been part of ADE training provider
	//TrainingJobServiceConfigByAlgo map[string]interface{}
	ModelDeployCmdTopic      string
	BaseTrainingDataLocalDir string
	ModelDir                 string // Remote Model dir hedge_export
	HedgeAdminURL            string
	EstimatedJobDuration     int64
	DigitalTwinUrl           string
	LocalDataStoreUrl        string // Defaults to Hedge
	DataStoreProvider        string // Was GCP earlier
	TrainingProvider         string // Was GCP earlier
	StorageBucket            string // StorageBucket is GCP specific, so needs to move as part of interface
	IsK8sEnv                 bool
	LocalDataCollection      bool
}

func NewMLMgmtConfig() *MLMgmtConfig {
	mlMgmtConfig := new(MLMgmtConfig)
	//mlMgmtConfig.TrainingJobServiceConfigByAlgo = make(map[string]interface{}, 0)
	return mlMgmtConfig
}

func (mlCfg *MLMgmtConfig) LoadCoreAppConfigurations(service interfaces.ApplicationService) error {
	lc := service.LoggingClient()

	dataStoreProvider, err := service.GetAppSetting("DataStore_Provider")
	if err != nil {
		lc.Errorf("DataStore_Provider is not configured, Default as Hedge(local victoria) will be assumed: %v", err)
		dataStoreProvider = "Hedge"
	}
	lc.Infof("dataStoreProvider : %s", dataStoreProvider)
	mlCfg.DataStoreProvider = dataStoreProvider

	localDataStoreURL, err := service.GetAppSetting("Local_DataStore_URL")
	if err != nil {
		lc.Errorf("Local_DataStore_URL is not configured, Default as Hedge(local victoria) will be assumed: %v", err)
		localDataStoreURL = "http://hedge-victoria-metrics:8428/api/v1"
	}
	lc.Infof("Local_DataSource_URL : %s", localDataStoreURL)
	mlCfg.LocalDataStoreUrl = localDataStoreURL

	modelDeployCmdTopic, err := service.GetAppSetting("ModelDeployTopic")
	if err != nil {
		lc.Error("error reading ModelDeployTopic from configuration")
		return err
	}
	lc.Infof("model deploy topic : %s", modelDeployCmdTopic)

	// BaseTrainingDataLocalDir
	baseTrainingDataLocalDir, err := service.GetAppSetting("BaseTrainingDataLocalDir")
	if err != nil {
		lc.Error("error reading BaseTrainingDataLocalDir, will assume /tmp")
		baseTrainingDataLocalDir = "/tmp"
	}
	lc.Infof("BaseTrainingDataLocalDir : %v", baseTrainingDataLocalDir)

	/*	storageBuckets, err := service.GetAppSetting("StorageBucket")
		if err != nil {
			lc.Warnf("GCP Storage bucket not configured, required if training provider is GCP: %v", err)
		} else {
			lc.Infof("StorageBucket : %v", storageBuckets)
		}*/

	modelDirs, err := service.GetAppSetting("ModelDir")
	if err == nil {
		// ModelDir is not required on broker side ie at edge
		lc.Info(fmt.Sprintf("ModelDir : %v", modelDirs))
		mlCfg.ModelDir = modelDirs
	}

	estimatedJobDuration, err := service.GetAppSetting("EstimatedJobDuration")
	if err != nil {
		lc.Error(err.Error())
		estimatedJobDuration = "1800"
	}
	lc.Info(fmt.Sprintf("EstimatedJobDuration : %v", estimatedJobDuration))

	jobDuration, _ := strconv.ParseInt(estimatedJobDuration, 0, 0)

	HedgeAdminURL, err := service.GetAppSetting("HedgeAdminURL")
	if err != nil {
		lc.Errorf("HedgeAdminURL is required to be configured: %v", err)
		mlCfg.HedgeAdminURL = "http://hedge-admin:48098"
	} else {
		lc.Infof("NodeURL : %s\n", HedgeAdminURL)
		mlCfg.HedgeAdminURL = HedgeAdminURL
	}

	digitalTwinUrl, err := service.GetAppSetting("DigitalTwinUrl")
	if err != nil {
		lc.Warnf("DigitalTwinUrl is required to be configured: %v", err)
		mlCfg.DigitalTwinUrl = "http://localhost:48090"
	} else {
		lc.Infof("DigitalTwinUrl: %s\n", digitalTwinUrl)
		mlCfg.DigitalTwinUrl = digitalTwinUrl
	}

	trainingProvider, err := service.GetAppSetting("Training_Provider")
	if err != nil {
		lc.Errorf("Training_Provider is not configured, Default as GCP will be assumed: %v", err)
		localDataStoreURL = "GCP"
	} else {
		lc.Infof("Training_Provider : %s", trainingProvider)
	}

	var isK8sEnv bool
	isK8sEnvStr := os.Getenv("IS_K8S_ENV")
	if isK8sEnvStr == "" {
		lc.Errorf("IS_K8S_ENV not correctly set in the env, set default=false : %v", err)
		isK8sEnv = false
	} else {
		isK8sEnv, err = strconv.ParseBool(isK8sEnvStr)
		if err != nil {
			lc.Errorf("IS_K8S_ENV has invalid value, set default=false. Parse error: %v", err)
			isK8sEnv = false
		} else {
			lc.Infof("IS_K8S_ENV : %t", isK8sEnv)
		}
	}

	if trainingProvider == "AIF" {
		var localDataCollection bool
		localDataCollectionStr, err := service.GetAppSetting("LocalDataCollection")
		if err != nil {
			lc.Errorf("LocalDataCollection not correctly set in the env, set default=false : %v", err)
			localDataCollection = false
		} else {
			localDataCollection, err = strconv.ParseBool(localDataCollectionStr)
			if err != nil {
				lc.Errorf("LocalDataCollection has invalid value, set default=false. Parse error: %v", err)
				localDataCollection = false
			} else {
				lc.Infof("LocalDataCollection : %t", localDataCollection)
			}
		}
		mlCfg.LocalDataCollection = localDataCollection
	} else {
		// Always localData collection from configured dataStore Provider
		mlCfg.LocalDataCollection = true
	}

	mlCfg.TrainingProvider = trainingProvider
	mlCfg.ModelDeployCmdTopic = modelDeployCmdTopic
	//mlCfg.StorageBucket = storageBuckets
	mlCfg.EstimatedJobDuration = jobDuration
	mlCfg.BaseTrainingDataLocalDir = baseTrainingDataLocalDir
	mlCfg.IsK8sEnv = isK8sEnv

	return nil
}
