/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"hedge/edge-ml-service/internal/training/hedge"
	"io/ioutil"
	"time"

	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	jobDefinitions "hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

// Added higher Status_monitor interval to allow AIF job to recover from pod failure stuff for now
const JOB_STATUS_MONITOR_INTERVAL time.Duration = 1.0 * 30 * time.Second
const JOB_STATUS_ELAPSED_TIME_INCREMENT int = 18
const JOB_STATUS_MONITOR_TIMEOUT = 4 * 60 * 60

var MonitorInterval time.Duration = 0
var ElapsedTimeIncrement int = 0
var MonitorTimeout int = 0

type JobExecutor struct {
	job                *jobDefinitions.TrainingJobDetails
	jobProviderService JobServiceProvider
	appConfig          *config.MLMgmtConfig
	service            interfaces.ApplicationService
	dbClient           redis.MLDbInterface
	mlStorage          helpers.MLStorageInterface
}

func NewJobExecutor(service interfaces.ApplicationService, appConfig *config.MLMgmtConfig, job *jobDefinitions.TrainingJobDetails, dbClient redis.MLDbInterface) *JobExecutor {
	jobExecutor := new(JobExecutor)
	jobExecutor.job = job
	jobExecutor.appConfig = appConfig
	jobExecutor.service = service
	jobExecutor.dbClient = dbClient
	jobExecutor.mlStorage = helpers.NewMLStorage(jobExecutor.appConfig.BaseTrainingDataLocalDir, jobExecutor.job.MLAlgorithm, jobExecutor.job.MLModelConfigName, service.LoggingClient())
	if appConfig.TrainingProvider == "Hedge" || appConfig.TrainingProvider == "" {
		hedgeJobSvcProvider := hedge.NewHedgeTrainingProvider(*appConfig, service, job)
		jobExecutor.jobProviderService = hedgeJobSvcProvider
	} else {
		service.LoggingClient().Errorf("No valid training provider configured, will not be able to do any training")
	}
	return jobExecutor
}

func (jobExecutor *JobExecutor) Execute() (*jobDefinitions.TrainingJobDetails, error) {

	//Code snippet to bypass for development if we are trying out model deployment and beyond
	/*	if 1 ==1 {
		jobExecutor.job.StatusCode = 4
		jobExecutor.dbClient.UpdateMLTrainingJob(*jobExecutor.job)
		return jobExecutor.job, nil
	}*/
	// Step-1: Upload training data
	lc := jobExecutor.service.LoggingClient()
	lc.Info("Start Executing training job")

	// get the training data file path and the config file path
	//mlStorage := helpers.NewMLStorage(jobExecutor.appConfig.BaseTrainingDataLocalDir, jobExecutor.job.MLAlgorithm, jobExecutor.job.MLModelConfigName, lc)

	localTrainingDataFileName := jobExecutor.mlStorage.GetTrainingDataFileName()
	// Check if file exist
	/*	if !jobExecutor.mlStorage.FileExists(localTrainingDataFileName) {
		jobExecutor.job.StatusCode = jobDefinitions.Failed
		jobExecutor.job.Msg = "Training data file missing :" + localTrainingDataFileName
		return jobExecutor.job, errors.New("Missing training data file: " + localTrainingDataFileName)
	}*/

	jobExecutor.job.StatusCode = jobDefinitions.UploadingTrainingData
	jobExecutor.dbClient.UpdateMLTrainingJob(*jobExecutor.job)
	// Copy config file to model directory
	jobExecutor.service.LoggingClient().Info("Will upload training data to training provider bucket, local data fileName : " + localTrainingDataFileName)

	// Step-2: Upload training data to Training Provider
	localConfigFileName := jobExecutor.mlStorage.GetMLModelConfigFileName(true)
	err := jobExecutor.uploadTrainingData(localTrainingDataFileName, localConfigFileName)
	if err != nil {
		jobExecutor.job.Msg = "Error while uploading the training data file :" + err.Error()
		jobExecutor.job.StatusCode = jobDefinitions.Failed // Failed StatusCode
		return jobExecutor.job, err
	}
	jobExecutor.service.LoggingClient().Info("Training data file uploaded to training provider")
	jobExecutor.job.StatusCode = 3 // TrainingInProgress
	jobExecutor.dbClient.UpdateMLTrainingJob(*jobExecutor.job)

	// Step-3: Submit Training job
	err = jobExecutor.submitTrainingJob() // Not sync, so handle it right
	if err != nil {
		jobExecutor.job.Msg = "Error while submitting the training job :" + err.Error()
		jobExecutor.job.StatusCode = jobDefinitions.Failed // Failed StatusCode
		return jobExecutor.job, err
	}
	jobExecutor.service.LoggingClient().Info("Training job submitted successfully to training provider")

	// Step-4: Monitor till the training job is completed or there is timeout
	// Poll for training to be completed
	err = jobExecutor.monitorTrainingJob()
	if err != nil {
		jobExecutor.job.Msg = "Error while monitoring the training job :" + err.Error()
		jobExecutor.job.StatusCode = jobDefinitions.Failed // Failed StatusCode
		return jobExecutor.job, err
	} else {
		err := jobExecutor.jobProviderService.DownloadModel(jobExecutor.mlStorage.GetBaseLocalDirectory(), "")
		if err != nil {
			jobExecutor.job.StatusCode = jobDefinitions.Failed
			jobExecutor.job.Msg = "Error while downloading the model & configuration to local file system :" + err.Error()
			return jobExecutor.job, err
		}
		jobExecutor.service.LoggingClient().Info("Model downloaded successfully")

		// Compress the downloaded file, no longer needed if the download is compressed
		/*		err = jobExecutor.mlStorage.CompressFolder(jobExecutor.mlStorage.GetModelLocalDir(), jobExecutor.mlStorage.GetModelZipFile())
				if err != nil {
					jobExecutor.job.StatusCode = 5
					jobExecutor.job.Msg = "Error compressing downloaded model folder :" + err.Error()
					return jobExecutor.job, err
				}*/

	}

	// Step-5: Publish to MQTT topic MLModel
	// Content will be job config: job json
	// We need to add download directory/URL
	/*
		if jobExecutor.appConfig.PublishModelReady {
				jobExecutor.service.LoggingClient().Info("Publish Model Ready Notification")
				err = jobExecutor.MqttEventGenerator()
				if err != nil {
					jobExecutor.job.Msg = "Error while MQTT Event Generator :" + err.Error()
					jobExecutor.job.StatusCode = 5 // Failed StatusCode
					return jobExecutor.job, err
				}
			}
	*/

	jobExecutor.job.StatusCode = jobDefinitions.TrainingCompleted
	jobExecutor.job.Msg = "Training completed successfully"
	jobExecutor.service.LoggingClient().Info(jobExecutor.job.Msg)

	return jobExecutor.job, nil
}

func (jobExecutor *JobExecutor) buildGCPTrainingConfigFileName() string {
	return jobExecutor.job.MLModelConfig.MLAlgorithm + "/" + jobExecutor.job.MLModelConfigName + "/assets/config.json"
}

func (jobExecutor *JobExecutor) buildConfigurationFile() (string, error) {
	// Read training data as per config from Victoria matrix and create a csv file
	trainingConfig := jobExecutor.job.MLModelConfig
	trgDataConfigBytes, _ := json.MarshalIndent(trainingConfig, "", " ")

	configFileName := jobExecutor.job.MLModelConfigName + "_" + "config.json"
	err := ioutil.WriteFile(configFileName, trgDataConfigBytes, 0644)
	if err != nil {
		return "", err
	}

	//trainingDataFile := jobExecutor.job.TrainingDataFileName

	return configFileName, nil

}

func (jobExecutor *JobExecutor) submitTrainingJob() error {
	// Submit the training job to GCP cloud for training and start monitoring for training status
	// Make sure we update the configuration for normalization multiplier per metric and save them
	// Also update threshold
	// Initiate polling for training and update the status of training and auto-download the model
	err := jobExecutor.jobProviderService.SubmitTrainingJob(jobExecutor.job)
	return err
}

func MonitorTrainingJob(lc logger.LoggingClient, appConfig *config.MLMgmtConfig, job *jobDefinitions.TrainingJobDetails,
	getJobStatusFunc func(*jobDefinitions.TrainingJobDetails) error) error {

	var monitorInterval time.Duration = JOB_STATUS_MONITOR_INTERVAL
	var elapsedTimeIncrement int = JOB_STATUS_ELAPSED_TIME_INCREMENT
	var monitorTimeout int = JOB_STATUS_MONITOR_TIMEOUT

	// for tests
	if MonitorInterval != 0 {
		monitorInterval = MonitorInterval
	}
	if ElapsedTimeIncrement != 0 {
		elapsedTimeIncrement = ElapsedTimeIncrement
	}
	if MonitorTimeout != 0 {
		monitorTimeout = MonitorTimeout
	}

	stop := false
	elapsedTimeSeconds := 0
	retryFailCount := 0
	for !stop {
		time.Sleep(monitorInterval)
		lc.Info("about to check training status")
		err := getJobStatusFunc(job)
		if err != nil || job.StatusCode >= 4 && job.StatusCode != 8 {
			lc.Info("training job completed ")
			retryFailCount++
			if (appConfig.TrainingProvider == "ADE" || appConfig.TrainingProvider == "AIF") && retryFailCount < 6 && (err != nil || job.StatusCode == 5) {
				// temp workaround since AIF job fails while creating the pod and then it succeeds in a minute or so
				lc.Infof("ade job failed, still retrying to check if it recovers from pod start delay")
				continue
			}
			stop = true
			if err != nil {
				return err
			} else if job.StatusCode > 4 {
				return errors.New(job.Msg)
			}
		}
		elapsedTimeSeconds += elapsedTimeIncrement
		if elapsedTimeSeconds > monitorTimeout {
			lc.Infof("forcefully marked the job complete with timeout failure; elapsedTimeSeconds: %d, monitorTimeout: %d", elapsedTimeSeconds, monitorTimeout)
			stop = true
			job.StatusCode = 6
			job.Msg = "Timed out while polling for job status"
		}
	}
	return nil
}

func (jobExecutor *JobExecutor) monitorTrainingJob() error {
	err := MonitorTrainingJob(jobExecutor.service.LoggingClient(), jobExecutor.appConfig, jobExecutor.job, jobExecutor.jobProviderService.GetTrainingJobStatus)
	return err
}

func (jobExecutor *JobExecutor) uploadTrainingData(localTrainingDataFileName string, localConfigFile string) error {

	if jobExecutor.appConfig.LocalDataCollection && !jobExecutor.mlStorage.FileExists(localTrainingDataFileName) {
		jobExecutor.job.StatusCode = jobDefinitions.Failed
		jobExecutor.job.Msg = "Training data file missing :" + localTrainingDataFileName
		return errors.New("Missing training data file: " + localTrainingDataFileName)
	}

	if jobExecutor.jobProviderService == nil {
		err := errors.New("training job provider service not configured, check your configuration for training_provider")
		jobExecutor.service.LoggingClient().Errorf("%s", err.Error())
		return err
	}

	//trainingDataFileNameRemote := "training_data.csv"
	// The filename is here needs to be aligned with what is defined in python training code
	//remoteTrainingDataFile := jobExecutor.job.TrainingDataConfig.MLAlgorithm + "/" + jobExecutor.job.MLModelConfigName + "/" + jobExecutor.appConfig.TrainingDataDir + "/" + trainingDataFileNameRemote

	jobExecutor.service.LoggingClient().Infof("About to upload training data & config file to training provider")

	err := jobExecutor.jobProviderService.UploadFile(localTrainingDataFileName, localConfigFile)
	if err != nil {
		jobExecutor.service.LoggingClient().Errorf("Error uploading training file: error %v", err)
		return err
	}

	return err
}
