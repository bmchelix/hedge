/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
	model "github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"hedge/common/client"
	mqttConfig "hedge/common/config"
	db2 "hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	"hedge/common/utils"
	trgConfig "hedge/edge-ml-service/internal/config"
	"hedge/edge-ml-service/internal/training/hedge"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/edge-ml-service/pkg/helpers"
	trainingdata "hedge/edge-ml-service/pkg/training-data"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type MqttSender interface {
	MQTTSend(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
}

const (
	MONITOR_DEPLOYMENT_STATUS_INTERVAL = 4 * time.Minute
	TERMINATE_JOB_INTERVAL             = 1 * time.Hour
	MAX_ANOMALY_THRESHOLD_VALUE        = 65535
)

type TrainingJobService struct {
	appConfig                            *config.MLMgmtConfig
	service                              interfaces.ApplicationService
	trainingConfigDataService            *trgConfig.MLModelConfigService
	trainingDataService                  TrainingDataServiceInterface
	dbClient                             redis.MLDbInterface
	currentConfigurationsWithRunningJobs map[string]struct{}
	lastProcessedTime                    int64
	mqttSender                           MqttSender
	telemetry                            *helpers.Telemetry
	client                               client.HTTPClient
	runningJobsMutex                     sync.Mutex
	connectionHandler                    interface{}
}

func IsTrainingJobInProgress(status job.JobStatus) bool {
	switch status {
	case job.TrainingDataCollected, job.UploadingTrainingData, job.TrainingInProgress:
		return true
	default:
		return false
	}
}

// Returns node specific topic
func BuildTopicNameForNode(
	topic string,
	ctx interfaces.AppFunctionContext,
	modelDeplCmd interface{},
) (string, error) {
	modelDeployCommand := modelDeplCmd.(*ml_model.ModelDeployCommand)
	topicName, err := mqttConfig.BuildTargetNodeTopicName(modelDeployCommand.TargetNodes[0], topic)
	return topicName, err
}

func NewTrainingJobService(
	service interfaces.ApplicationService,
	appConfig *config.MLMgmtConfig,
	trainingConfigDataService *trgConfig.MLModelConfigService,
	dbClient redis.MLDbInterface,
	trainingDataService TrainingDataServiceInterface,
	telemetry *helpers.Telemetry,
	mqttSender MqttSender,
	connectionHandler interface{},
) *TrainingJobService {
	trainingJobService := new(TrainingJobService)
	trainingJobService.service = service
	trainingJobService.appConfig = appConfig
	trainingJobService.trainingConfigDataService = trainingConfigDataService
	trainingJobService.trainingDataService = trainingDataService
	trainingJobService.dbClient = dbClient
	trainingJobService.telemetry = telemetry
	trainingJobService.currentConfigurationsWithRunningJobs = make(map[string]struct{})
	trainingJobService.connectionHandler = connectionHandler

	if mqttSender == nil {
		config, err := mqttConfig.BuildMQTTSecretConfig(
			service,
			appConfig.ModelDeployCmdTopic,
			client.HedgeMLManagementServiceName,
		)
		if err != nil {
			service.LoggingClient().Errorf("Error Building MQTT client: Error: %v", err)
		} else {

			MQTTSender := transforms.NewMQTTSecretSenderWithTopicFormatter(config, mqttConfig.GetPersistOnError(service), BuildTopicNameForNode)
			trainingJobService.mqttSender = MQTTSender
		}
	} else {
		trainingJobService.mqttSender = mqttSender // for testing purpose only
	}
	// Register ML Algo template and its instance in AIF deployed on Helix SaaS

	return trainingJobService
}

func (js *TrainingJobService) SetHttpClient() {
	js.client = &http.Client{}
}

/*
// Make sure HedgeAnomaly is registered as algo template
// For each hedge Algo, update the jobServiceConfig (templateId and connection) in appConfig
// For now create the template if it is not found, later consider auto-update
func (js *TrainingJobService) RegisterTrainingProviders() error {
	if js.appConfig.TrainingProvider == "ADE" || js.appConfig.TrainingProvider == "AIF" {
		// TODO: Girish, fix it so mlAlgo name is taken from configuration, JobScript.yaml and template file also need to be changed/generated

		//adeJobServiceConfigs, err := ade.RegisterTrainingProvider(
		//	"HedgeAnomaly",
		//	js.service,
		//	js.connectionHandler.(commService.ADEConnectionInterface),
		//	"res/ade/hedge_algo_template.json",
		//)
		//if err == nil && adeJobServiceConfigs != nil && len(adeJobServiceConfigs) > 0 {
		//	js.service.LoggingClient().
		//		Infof("Successfully registered Hedge Algo template to ADE AIF: %+v", adeJobServiceConfigs)
		//	js.appConfig.TrainingJobServiceConfigByAlgo = adeJobServiceConfigs
		//	js.service.LoggingClient().
		//		Infof("Training Job Service Config: %+v", js.appConfig.TrainingJobServiceConfigByAlgo)
		//} else {
		//	js.service.LoggingClient().Errorf("Failed to register Hedge Algo template to ADE AIF")
		//	return err
		//}

		adeDataCollectorJobServiceConfigs, err := ade.RegisterTrainingProvider(helpers.DATA_COLLECTOR_NAME, js.service, js.connectionHandler.(commService.ADEConnectionInterface), "res/ade/hedge_algo_template_dataCollector.json", "res/ade/jobScript_dataCollector.yml")
		if err == nil && adeDataCollectorJobServiceConfigs != nil && len(adeDataCollectorJobServiceConfigs) > 0 {
			js.service.LoggingClient().Infof("Successfully registered DataCollector template to ADE AIF: %+v", adeDataCollectorJobServiceConfigs)
			js.appConfig.TrainingJobServiceConfigByAlgo[helpers.DATA_COLLECTOR_NAME] = adeDataCollectorJobServiceConfigs[helpers.DATA_COLLECTOR_NAME]
		} else {
			js.service.LoggingClient().Errorf("Failed to register DataCollector template to ADE AIF")
			return err
		}
		return nil
	}
	// Nothing specific to GCP for now as per current implementation, so need to add that
	return nil
}*/

// BuildJobConfig
// May throw hedgeError Not Found
func (js *TrainingJobService) BuildJobConfig(
	jobSubmissionDetails job.JobSubmissionDetails,
) (*job.TrainingJobDetails, hedgeErrors.HedgeError) {

	var err hedgeErrors.HedgeError
	lc := js.service.LoggingClient()
	defer lc.Infof("Returning from %s", "BuildJobConfig")

	logger := js.service.LoggingClient()

	mlModelConfig, err := js.trainingConfigDataService.GetMLModelConfig(
		jobSubmissionDetails.MLAlgorithm,
		jobSubmissionDetails.MLModelConfigName,
	)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return nil, err
	}

	logger.Infof(
		"training config: %+v, training provider: %s",
		mlModelConfig,
		js.appConfig.TrainingProvider,
	)

	algorithmConfig, err := js.trainingConfigDataService.GetAlgorithm(jobSubmissionDetails.MLAlgorithm)
	if err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			fmt.Sprintf("error getting algorithm: %s", jobSubmissionDetails.MLAlgorithm))
	}

	logger.Infof("training provider: %s", js.appConfig.TrainingProvider)
	switch js.appConfig.TrainingProvider {

	case "Hedge", "":
		hedgeTrgJobConfig, err := hedge.BuildJobConfig(jobSubmissionDetails, algorithmConfig, mlModelConfig, js.service)
		return hedgeTrgJobConfig, err
	default:
		//logger.Infof("unsupported training provider: %s", js.appConfig.TrainingProvider)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("Unsupported training provider: %s", js.appConfig.TrainingProvider),
		)
	}
	return nil, nil
}

func (js *TrainingJobService) getLockKey(inputJob *job.TrainingJobDetails) string {
	return inputJob.MLAlgorithm + "_" + inputJob.MLModelConfigName
}

func (js *TrainingJobService) SubmitJob(inputJob *job.TrainingJobDetails) hedgeErrors.HedgeError {
	// Add the model if it is not present so it can be managed

	if !inputJob.MLModelConfig.Enabled {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			fmt.Sprintf(
				"Training Data Configuration: %s not enabled, so not starting the job",
				inputJob.MLModelConfigName,
			),
		)
	}

	js.runningJobsMutex.Lock()
	lockKey := js.getLockKey(inputJob)
	if _, found := js.currentConfigurationsWithRunningJobs[lockKey]; found {
		js.runningJobsMutex.Unlock()
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConflict,
			fmt.Sprintf(
				"Training Job with Configuration: %s already running, Please wait for the job to complete",
				inputJob.MLModelConfigName,
			),
		)
	}
	jobDtls, err := js.addJob(inputJob)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		js.runningJobsMutex.Unlock()
		return err
	}

	// Check the trainingProvider to decide where to submit the job
	jobExecutor := NewJobExecutor(js.service, js.appConfig, inputJob, js.dbClient)

	if jobExecutor == nil || jobExecutor.jobProviderService == nil {
		js.runningJobsMutex.Unlock()
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Job Executor returned nil, check configuration of the training provider",
		)
	}

	js.currentConfigurationsWithRunningJobs[lockKey] = struct{}{}
	js.runningJobsMutex.Unlock()

	// Submit to this channel function and don't wait
	go func(notify bool, simulationDefinitionName string) {
		// Check if the training data file was already generated or we need to generate afresh

		// existingJob, err := js.dbClient.GetMLTrainingJob(job.Name)
		// if err == db2.ErrNotFound {
		var trainingDataFileId string
		if jobDtls.StatusCode != job.TrainingDataCollected { // ie not equals TrainingDataCollected
			// Job doesn't exist or was errored out, so create the training data afresh
			jobSubmissionDetails := job.JobSubmissionDetails{
				Name:              jobDtls.Name,
				MLAlgorithm:       inputJob.MLAlgorithm,
				MLModelConfigName: inputJob.MLModelConfigName,
				UseUploadedData:   inputJob.UseUploadedData,
			}
			if js.appConfig.LocalDataCollection {
				trainingDataFileId, err = js.trainingDataService.CreateTrainingDataLocal(
					inputJob.MLModelConfig,
					&jobSubmissionDetails,
				)
				jobDtls.TrainingDataFileId = trainingDataFileId
			}
			if err != nil || !js.appConfig.LocalDataCollection {
				if !js.appConfig.LocalDataCollection {
					js.service.LoggingClient().Error("LocalDataCollection is set as false, not supported")
				}
				js.service.LoggingClient().
					Errorf("Terminating the training job as the data file generation returned error: %v\n", err)
				js.service.LoggingClient().
					Errorf("Terminating the training job as the data file generation returned error: %v\n", err)
				js.runningJobsMutex.Lock()
				delete(js.currentConfigurationsWithRunningJobs, lockKey)
				js.runningJobsMutex.Unlock()
				return
			}
		} else {
			trainingDataFileId = jobDtls.TrainingDataFileId
		}

		// Now go submit the job to training provider with the data file already in place now
		var jobDetails *job.TrainingJobDetails
		var executeErr error
		jobDetails, executeErr = jobExecutor.Execute()
		/*		if js.appConfig.LocalDataCollection {
					jobDetails, executeErr = jobExecutor.Execute()
				} else {
					jobDetails, executeErr = jobExecutor.ExecuteDeleteMe(trainingDataFileId)
				}*/
		if executeErr != nil {
			js.service.LoggingClient().Warn(executeErr.Error())
		}

		var modelVersion int64 = 0
		// Only if the job created and downloaded the model successfully do we update the model and provide a new version to it
		if jobDetails.StatusCode == 4 {
			js.service.LoggingClient().
				Infof("Trained model ready, will register the model and model version")
			// If there is a previous model, update the version
			modelVersion, err = js.dbClient.GetLatestModelVersion(
				inputJob.MLAlgorithm,
				inputJob.MLModelConfigName,
			)
			if err != nil {
				js.service.LoggingClient().
					Warnf("Error getting models before registering the model, error:%v", err)
				modelVersion = 1
			} else {
				modelVersion += 1
			}

			// Check the deployment with latest model version, if there is no deployment, keep the same version
			/* existingDeployments, err := js.dbClient.GetDeploymentsByConfig(inputJob.MLAlgorithm, inputJob.MLModelConfigName)
			if err == nil || errors.Is(err, db2.ErrNotFound) {
				// Get the latest version from existing deployments
				for _, depl := range existingDeployments {
					if depl.ModelVersion >= modelVersion {
						modelVersion += 1
					}
				}
			}
			*/

			// Add model to model db
			mlModel := ml_model.MLModel{
				MLAlgorithm:          inputJob.MLAlgorithm,
				MLModelConfigName:    inputJob.MLModelConfigName,
				ModelName:            inputJob.Name,
				ModelVersion:         modelVersion,
				ModelCreatedTimeSecs: time.Now().Unix(),
			}
			err = js.dbClient.SaveMLModel(mlModel)
			if err != nil {
				js.service.LoggingClient().
					Errorf("Error saving the model: %s, error: %v", inputJob.Name, err)
				inputJob.Msg = "Failed to save the model"
				inputJob.StatusCode = 5 // Failed
			} else {
				js.service.LoggingClient().Infof("Model %s, Version: %d saved", inputJob.MLModelConfigName, modelVersion)
				jobDetails.ModelNameVersion = fmt.Sprintf("%s:v%d", inputJob.Name, modelVersion)
				jobDetails.Msg = "Training completed and model saved successfully"
				// Now create a default AnomalyEvent Configuration for the new model that just got generated
				// We copy the last version if that is available or we create a default one

				_ = js.AddDefaultMLEventConfig(jobDetails.MLModelConfig, inputJob.ModelName)
			}
		}
		// Remove the job since it is complete either successfully or failed
		js.runningJobsMutex.Lock()
		delete(js.currentConfigurationsWithRunningJobs, lockKey)
		js.runningJobsMutex.Unlock()

		jobDetails.EndTime = time.Now().Unix()
		jobDetails.SimulationDefinitionName = simulationDefinitionName

		_, _ = js.dbClient.UpdateMLTrainingJob(*jobDetails)

		js.service.LoggingClient().Infof("Sending support notification for %s", inputJob.Name)

		serialized, err := json.Marshal(jobDetails)
		if err != nil {
			js.service.LoggingClient().Errorf("Error marshaling jobDetails: %v", err)
		} else {
			js.service.LoggingClient().Infof("Marshaled jobDetails: %s", string(serialized))
		}

		if notify {

			// Save the event config to the DB
			config := config.MLEventConfig{
				MLAlgorithm:                inputJob.MLAlgorithm,
				MlModelConfigName:          inputJob.MLModelConfigName,
				StabilizationPeriodByCount: 1,
			}
			_, err = js.dbClient.AddMLEventConfig(config)
			if err != nil {
				js.service.LoggingClient().Errorf("Error saving EventConfig: %v", err)
			}

			// Send a support notification if requested
			notification := dtos.Notification{
				DBTimestamp: dtos.DBTimestamp{},
				Category:    "deployment-finished",
				Sender:      "ml-management",
				Content:     string(serialized),
				ContentType: "application/json",
				Severity:    model.Normal,
				Status:      "NEW",
				Labels:      []string{"daily"},
			}

			req := requests.NewAddNotificationRequest(notification)
			responses, err := js.service.NotificationClient().
				SendNotification(js.service.AppContext(), []requests.AddNotificationRequest{req})
			if err != nil {
				js.service.LoggingClient().Errorf("error publishing notificaton: %s", err.Error())
			}
			if len(responses) > 0 {
				js.service.LoggingClient().
					Infof("responses from support notification publish request: %+v", responses)
			}
		}

		err1 := js.telemetry.ProcessJob(jobDtls)
		if err1 != nil {
			js.service.LoggingClient().
				Errorf("Error calculating ML metrics counters, error: %v", err1.Error())
		}
	}(inputJob.NotifyOnCompletion, inputJob.SimulationDefinitionName)

	return err
}

func (js *TrainingJobService) RegisterUploadedModel(
	tmpFilePath string,
	tmpFile multipart.File,
	inputJob *job.TrainingJobDetails,
) hedgeErrors.HedgeError {
	js.runningJobsMutex.Lock()
	lockKey := js.getLockKey(inputJob)
	if _, found := js.currentConfigurationsWithRunningJobs[lockKey]; found {
		js.runningJobsMutex.Unlock()
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConflict,
			fmt.Sprintf(
				"Job with Configuration: %s already running, Please wait for the job to complete",
				inputJob.MLModelConfigName,
			),
		)
	}

	errMsg := fmt.Sprintf("Failed to register uploaded model %s", inputJob.MLModelConfigName)

	js.currentConfigurationsWithRunningJobs[lockKey] = struct{}{}
	js.runningJobsMutex.Unlock()
	defer func() {
		js.runningJobsMutex.Lock()
		delete(js.currentConfigurationsWithRunningJobs, lockKey)
		js.runningJobsMutex.Unlock()
	}()

	dst, err := os.Create(tmpFilePath)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}
	defer dst.Close()
	// Copy the uploaded file to the destination file
	if _, err = io.Copy(dst, tmpFile); err != nil {
		js.service.LoggingClient().Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	err = validateUploadedModel(tmpFilePath)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	jobDtls, err := js.addJob(inputJob)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}
	mlStorage := helpers.NewMLStorage(
		js.appConfig.BaseTrainingDataLocalDir,
		inputJob.MLAlgorithm,
		inputJob.MLModelConfigName,
		js.service.LoggingClient(),
	)
	modelFilePath := mlStorage.GetModelLocalZipFileName()

	if _, err := os.Stat(modelFilePath); err == nil {
		// File exists, so remove it
		if err := os.Remove(modelFilePath); err != nil {
			js.service.LoggingClient().Error(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
		}
	}

	err = os.Rename(tmpFilePath, modelFilePath)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	jobDtls.StartTime = time.Now().Unix()
	jobDtls.StatusCode = 4
	var modelVersion int64 = 0
	modelVersion, err = js.dbClient.GetLatestModelVersion(
		inputJob.MLAlgorithm,
		inputJob.MLModelConfigName,
	)
	if err != nil {
		js.service.LoggingClient().
			Warnf("Error getting models before registering the uploaded model, error:%v", err)
		modelVersion = 1
	} else {
		modelVersion += 1
	}

	mlModel := ml_model.MLModel{
		MLAlgorithm:          inputJob.MLAlgorithm,
		MLModelConfigName:    inputJob.MLModelConfigName,
		ModelName:            inputJob.Name,
		ModelVersion:         modelVersion,
		ModelCreatedTimeSecs: time.Now().Unix(),
	}
	err = js.dbClient.SaveMLModel(mlModel)
	if err != nil {
		js.service.LoggingClient().
			Errorf("Error saving the uploaded model: %s, error: %v", inputJob.Name, err)
		jobDtls.Msg = "Failed to save the uploaded model"
		jobDtls.StatusCode = 5 // Failed
	} else {
		js.service.LoggingClient().Infof("Model %s, Version: %d saved", inputJob.MLModelConfigName, modelVersion)
		jobDtls.ModelNameVersion = fmt.Sprintf("%s:v%d", inputJob.Name, modelVersion)
		jobDtls.Msg = "Uploaded model saved successfully"
		// Now create a default AnomalyEvent Configuration for the new model that just got generated
		// We copy the last version if that is available or we create a default one

		_ = js.AddDefaultMLEventConfig(jobDtls.MLModelConfig, inputJob.ModelName)
	}

	jobDtls.EndTime = time.Now().Unix()

	_, _ = js.dbClient.UpdateMLTrainingJob(*jobDtls)

	err1 := js.telemetry.ProcessJob(jobDtls)
	if err1 != nil {
		js.service.LoggingClient().
			Errorf("Error calculating ML metrics counters, error: %v", err1.Error())
	}

	return nil
}

// Adds the job to the database for tracking purpose
func (js *TrainingJobService) addJob(
	job *job.TrainingJobDetails,
) (*job.TrainingJobDetails, hedgeErrors.HedgeError) {

	configJobsKey := db2.MLTrainingJob + ":config:" + job.MLModelConfigName

	duration, err := js.GetEstimatedDuration(configJobsKey)

	job.EstimatedDuration = duration

	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return nil, err
	}

	existingJob, err := js.dbClient.GetMLTrainingJob(job.Name)
	if err == nil &&
		(existingJob.StatusCode <= 1 || existingJob.StatusCode == 5 || existingJob.StatusCode == 6) {
		// Let this continue and overwrite this job
		// Update with new date etc
		job.StatusCode = existingJob.StatusCode
		_, _ = js.dbClient.UpdateMLTrainingJob(*job)
		return job, nil
	}
	_, err = js.dbClient.AddMLTrainingJob(*job)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return nil, err
	}

	return job, nil
}

func (js *TrainingJobService) GetJobs(
	jobKey string,
) ([]job.TrainingJobDetails, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error getting job %s", jobKey)

	jobList, err := js.dbClient.GetMLTrainingJobs(jobKey)
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return jobList, err
	}

	sort.Slice(jobList, func(i, j int) bool {
		return jobList[i].StartTime > jobList[j].StartTime
	})

	return jobList, nil
}

func (js *TrainingJobService) GetJob(
	jobName string,
) (job.TrainingJobDetails, hedgeErrors.HedgeError) {
	var job job.TrainingJobDetails
	var err hedgeErrors.HedgeError
	job, err = js.dbClient.GetMLTrainingJob(jobName)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return job, err
	}
	return job, nil
}

func (js *TrainingJobService) DeleteMLTrainingJob(jobName string) hedgeErrors.HedgeError {
	job, err := js.GetJob(jobName)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return err
	}
	if job.StatusCode <= 1 || job.StatusCode == 5 || job.StatusCode == 6 {
		err = js.dbClient.DeleteMLTrainingJobs(jobName)
		if err != nil {
			js.service.LoggingClient().Error(err.Error())
		}
	} else {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "Training job cannot be deleted")
	}

	return nil
}

func (js *TrainingJobService) GetEstimatedDuration(
	configJobsKey string,
) (int64, hedgeErrors.HedgeError) {
	var sum int64
	var count int64
	jobs, err := js.GetJobs(configJobsKey)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return 0, err
	}
	for _, job := range jobs {
		if job.StatusCode == 4 {
			sum += job.EndTime - job.StartTime
			count += 1
		}
	}
	if count == 0 {
		return js.appConfig.EstimatedJobDuration, nil
	}
	return sum / count, nil
}

func (js *TrainingJobService) GetModels(
	mlAlgorithm string,
	mlModelConfigName string,
	latestVersionsOnly bool,
) ([]ml_model.MLModel, hedgeErrors.HedgeError) {
	var models []ml_model.MLModel
	var err hedgeErrors.HedgeError

	lc := js.service.LoggingClient()

	if !latestVersionsOnly {
		models, err = js.dbClient.GetModels(mlAlgorithm, mlModelConfigName)
	} else {
		models, err = js.dbClient.GetLatestModelsByConfig(mlAlgorithm, mlModelConfigName)
	}

	if err != nil {
		lc.Errorf("Error getting models: %v", err)
		return nil, err
	}

	return models, nil
}

func (js *TrainingJobService) ValidateModelVersionExist(
	mlAlgorithm string,
	mlModelConfigName string,
	modelVersion int64,
) hedgeErrors.HedgeError {
	lc := js.service.LoggingClient()

	errorMessage :=
		fmt.Sprintf(
			"Error validating existence of model version %d algorithm %s, configName %s",
			modelVersion,
			mlAlgorithm,
			mlModelConfigName,
		)

	// Ensure that trainingConfig exist
	_, err := js.dbClient.GetMlModelConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return err
	}

	models, err := js.dbClient.GetModels(mlAlgorithm, mlModelConfigName)

	if len(models) == 0 || err != nil {
		lc.Errorf("%s: Not found", errorMessage)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			fmt.Sprintf("No model found for algorithm %s", mlAlgorithm),
		)
	}

	for _, model := range models {
		if model.ModelVersion == modelVersion {
			return nil
		}
	}
	// TODO: shouldn't be returned error??
	return nil
}

// GetModelDeploymentsByConfig for all trained models given mlModelConfig,
// It also gets deployments for nodes. If there is no deployment against a node, it still gets the entry for the node without deployment
func (js *TrainingJobService) GetModelDeploymentsByConfig(
	mlAlgorithm string,
	mlModelConfigName string,
	groupByDeployment bool,
) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError) {

	deployments := make([]ml_model.ModelDeploymentStatus, 0)
	retreivedModels, err := js.dbClient.GetModels(mlAlgorithm, mlModelConfigName)
	if err != nil {
		// mostly the singleModel doesn't exist as yet, so continue
		js.service.LoggingClient().
			Warnf("No singleModel or error for training config: %s, error: %v", mlModelConfigName, err)
		return deployments, err
	}

	sort.Slice(retreivedModels, func(i, j int) bool {
		if retreivedModels[i].MLModelConfigName+fmt.Sprintf(
			"%d",
			retreivedModels[i].ModelVersion,
		) >= retreivedModels[j].MLModelConfigName+fmt.Sprintf(
			"%d",
			retreivedModels[j].ModelVersion,
		) {
			return true
		}
		return retreivedModels[i].ModelCreatedTimeSecs > retreivedModels[j].ModelCreatedTimeSecs
	})
	js.service.LoggingClient().Debugf("sorted the singleModel, size = %d", len(retreivedModels))
	deploymentsByConfig, err := js.dbClient.GetDeploymentsByConfig(mlAlgorithm, mlModelConfigName)
	if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		// Real error that we can't recover
		js.service.LoggingClient().Errorf("Error reading the deploymentsByConfig Error: %v", err)
		return deployments, err
	}
	// Now do an outer join of retreivedModels & deployments grouped by singleModel, version and deploymentStatus

	// var matchingDeployment *ml_model.ModelDeploymentStatus
	nodes, _ := js.getNodes()
	// Map of trg config -> set of nodes
	config2NodesSet := make(map[string]map[string]struct{})
	for _, singleModel := range retreivedModels {
		if _, ok := config2NodesSet[singleModel.MLModelConfigName]; !ok {
			config2NodesSet[singleModel.MLModelConfigName] = make(map[string]struct{})
		}

		// Find matching deployments
		js.service.LoggingClient().Debugf("processing singleModel: %v", singleModel)

		for _, deployment := range deploymentsByConfig {
			if singleModel.MLAlgorithm == deployment.MLAlgorithm &&
				singleModel.MLModelConfigName == deployment.MLModelConfigName &&
				deployment.ModelVersion == singleModel.ModelVersion {

				config2NodesSet[singleModel.MLModelConfigName][deployment.NodeId] = struct{}{}

				if singleModel.IsModelDeprecated {
					deployment.IsModelDeprecated = true
				}
				if groupByDeployment {
					// Check if we find in deployments, if so just add into it

					// get all nodes here

					found := false
					for i, currentDepl := range deployments {
						if currentDepl.MLModelConfigName == deployment.MLModelConfigName &&
							currentDepl.ModelVersion == deployment.ModelVersion &&
							currentDepl.DeploymentStatusCode == deployment.DeploymentStatusCode {
							// We append the list of deployments for deployment summary
							deployments[i].NodeId = fmt.Sprintf(
								"%s, %s",
								currentDepl.NodeId,
								deployment.NodeId,
							)

							for _, node := range nodes {
								if node.NodeId == deployments[i].NodeId {
									if node.HostName != "" {
										deployments[i].NodeDisplayName = node.HostName
									} else {
										deployments[i].NodeDisplayName = node.Name
									}
									break
								}
							}
							if deployments[i].NodeDisplayName == "" {
								deployments[i].NodeDisplayName = deployments[i].NodeId
							}
							found = true
							// break
						}
					}
					if !found {
						// Temp for now
						if deployment.NodeDisplayName == "" {
							// not sure why this path...
							deployment.NodeDisplayName = deployment.NodeId
						}
						deployments = append(deployments, deployment)
					}
				} else {
					deployments = append(deployments, deployment)
				}
			}
		}

		// js.addMissingNodes(nodes, config2NodeInclusionSet[model.MLModelConfigName], model ml_model.MLModel)
		// Take care of missing nodes when few nodes are considered, but new ones are left out in the list the user sees
		ready2DeployRowPresent := false
		for _, node := range nodes {
			if _, ok := config2NodesSet[singleModel.MLModelConfigName][node.NodeId]; !ok {
				if !ready2DeployRowPresent {
					newDeply := buildNewDeploymentStatusRecord(singleModel, node)
					deployments = append(deployments, newDeply)
					ready2DeployRowPresent = true
				} else if groupByDeployment {
					// We already have deployment for the config, but this node was absent, so just append it for UI display
					deployments[len(deployments)-1].NodeId = fmt.Sprintf("%s, %s", deployments[len(deployments)-1].NodeId, node.NodeId)
					deployments[len(deployments)-1].NodeDisplayName = fmt.Sprintf("%s, %s", deployments[len(deployments)-1].NodeDisplayName, node.HostName)
				}
				js.service.LoggingClient().
					Debugf("No matching deployments, adding a new one in ReadyToDeploy status for node: %v", node.NodeId)
				config2NodesSet[singleModel.MLModelConfigName][node.NodeId] = struct{}{}
			}
		}
	}

	return deployments, nil
}

// GetModelDeploymentsGroupedByNode Get Models and Deployments grouped by Nodes, filter by NodeName not implemented as of now, may not be required as of now, []ml_model.DeploymentsByGroup
func (js *TrainingJobService) GetModelDeploymentsGroupedByNode(algorithm string, trainingConfig string, nodeName string, deploymentStatus string) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError) {

	// Get list of nodes first
	nodes, hErr := js.getNodes()
	if hErr != nil {
		js.service.LoggingClient().Errorf("Error getting nodeGroups %v", hErr)
		return nil, hErr
	}
	// Get all modeldeployments even if there is no deployment for model
	modelDeployments, err := js.GetModelDeploymentsByConfig(algorithm, trainingConfig, false)
	if err != nil {
		js.service.LoggingClient().Errorf("Error getting DeploymentsByConfig %v", err)
		return nil, err
	}

	// if we want to get by status, we should not get only the latest versions otherwise we miss
	// already deployed model status entries

	if deploymentStatus == "" {
		// Get valid algo+version combination from above
		latestModels, err := js.GetModels(algorithm, trainingConfig, true)
		if err != nil {
			js.service.LoggingClient().Errorf("Error getting latest Models %v", err)
			return nil, err
		}
		latestModelVersionKeys := buildLatestModelVersionKeys(latestModels)

		deploymentsByNode := make([]ml_model.ModelDeploymentStatus, 0)
		// node to deployment array
		for _, node := range nodes {
			nodeDeployments := buildNodeDeployments(latestModelVersionKeys, modelDeployments, node)
			if nodeDeployments != nil {
				deploymentsByNode = append(deploymentsByNode, nodeDeployments...)
			}
		}
		return deploymentsByNode, nil
	} else {

		modelDeploymentsFilteredByStatus := make([]ml_model.ModelDeploymentStatus, 0)
		for _, modelDeployment := range modelDeployments {
			if modelDeployment.DeploymentStatus == deploymentStatus {
				modelDeploymentsFilteredByStatus = append(
					modelDeploymentsFilteredByStatus,
					modelDeployment,
				)
			}
		}
		modelDeployments = modelDeploymentsFilteredByStatus
		return modelDeployments, nil
	}
}

// Build node with deployment. If deployment is missing, have an empty model against it indicating ready to deploy
func buildNodeDeployments(
	latestModelVersionKeys []string,
	deployments []ml_model.ModelDeploymentStatus,
	node models.Node,
) []ml_model.ModelDeploymentStatus { // []ml_model.DeploymentsByGroup
	modelDeployments := make([]ml_model.ModelDeploymentStatus, 0)
	// For every node, attach the candidate deployments if not already part of deployment
	for _, modelAndVersionKey := range latestModelVersionKeys {
		// Find the matching deployments and add to modelVersionGrp#Deployments
		// based on nodeName & ModelVersion
		modelDeploymnt := buildMatchingDeployment(
			deployments,
			modelAndVersionKey,
			node.NodeId,
			node.HostName,
		)
		if modelDeploymnt != nil {
			modelDeployments = append(modelDeployments, *modelDeploymnt)
		}
	}
	return modelDeployments
}

// Build node tree without deployments
func buildNodeTree(
	nodeGroup *models.NodeGroup,
	latestModelVersionKeys []string,
	deployments []ml_model.ModelDeploymentStatus,
	leafNodeName string,
) interface{} { // []ml_model.DeploymentsByGroup

	if nodeGroup.Node != nil {
		// add algorithm node

		modelDeployments := make([]ml_model.ModelDeploymentStatus, 0)
		// For every node, attach the candidate deployments if not already part of deployment
		for _, modelAndVersionKey := range latestModelVersionKeys {
			// Find the matching deployments and add to modelVersionGrp#Deployments
			// based on nodeName & ModelVersion
			modelDeploymnt := buildMatchingDeployment(
				deployments,
				modelAndVersionKey,
				nodeGroup.Node.NodeId,
				nodeGroup.DisplayName,
			)
			if modelDeploymnt != nil {
				modelDeployments = append(modelDeployments, *modelDeploymnt)
			}
		}

		return modelDeployments
	} else {
		// add node group and call recursively for each childGroup
		deploymentGrp := ml_model.DeploymentsByGroup{
			GroupField:       "NodeGroup",
			GroupValue:       nodeGroup.DisplayName,
			DeploymentGroups: nil,
		}

		// childDeployedGroups := make([]ml_model.DeploymentsByGroup, 0)
		childDeployedGroups := make([]interface{}, 0)
		for _, childNodeGroup := range nodeGroup.ChildNodeGroups {
			newNodes := buildNodeTree(&childNodeGroup, latestModelVersionKeys, deployments, "")
			if newNodes == nil {
				continue
			}
			nodeTreeType := reflect.TypeOf(newNodes).String()
			if nodeTreeType == "[]ml_model.DeploymentsByGroup" {
				for _, newNode := range newNodes.([]ml_model.DeploymentsByGroup) {
					childDeployedGroups = append(childDeployedGroups, newNode)
				}
			} else {
				for _, newNode := range newNodes.([]ml_model.ModelDeploymentStatus) {
					childDeployedGroups = append(childDeployedGroups, newNode)
				}
			}

		}
		deploymentGrp.DeploymentGroups = childDeployedGroups
		return []ml_model.DeploymentsByGroup{deploymentGrp}
	}
}

func buildMatchingDeployment(
	deployments []ml_model.ModelDeploymentStatus,
	modelAndVersionKey string,
	nodeName string,
	nodeDisplayName string,
) *ml_model.ModelDeploymentStatus {
	configKeyArray := strings.Split(modelAndVersionKey, ":v")
	// We will never have this case
	if len(configKeyArray) == 0 {
		return nil
	}
	configKey := configKeyArray[0]

	// First try to get the deployed version
	for _, deply := range deployments {
		// deplModelVersion := deply.MLModelConfigName + ":v" + fmt.Sprintf("%d", deply.ModelVersion)
		if deply.NodeId == nodeName && configKey == deply.MLModelConfigName {
			deply.NodeDisplayName = nodeDisplayName
			return &deply
		}
	}
	// If no deployments, get the latest deployment candidate and go with it
	for _, deply := range deployments {
		deplModelVersion := deply.MLModelConfigName + ":v" + fmt.Sprintf("%d", deply.ModelVersion)
		if deplModelVersion == modelAndVersionKey {
			// Modify deply and return
			deply.NodeId = nodeName
			deply.NodeDisplayName = nodeDisplayName
			deply.DeploymentStatusCode = ml_model.ReadyToDeploy
			deply.DeploymentStatus = ml_model.ReadyToDeploy.String()
			deply.ModelDeploymentTimeSecs = 0
			deply.PermittedOption = "deploy"
			return &deply
		}
	}

	return nil
}

func (js *TrainingJobService) getNodesGroups() ([]models.NodeGroup, hedgeErrors.HedgeError) {
	url := js.appConfig.HedgeAdminURL + "/api/v3/node_mgmt/group/all"
	js.service.LoggingClient().Infof("Getting list of nodes from URL: %s\n", url)

	errorMessage := "Failed to fetch nodes group"

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		js.service.LoggingClient().
			Warnf("Error creating http request to to get node list %s, %v", url, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := js.client.Do(request)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	var nodeGroups []models.NodeGroup
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	err = json.Unmarshal(bytes, &nodeGroups)
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return nodeGroups, nil
}

func (js *TrainingJobService) getNodes() ([]models.Node, hedgeErrors.HedgeError) {
	url := js.appConfig.HedgeAdminURL + "/api/v3/node_mgmt/node/all"
	js.service.LoggingClient().Debugf("Getting list of nodes from URL: %s\n", url)
	var nodes []models.Node

	errorMessage := "Error getting list of nodes"

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		js.service.LoggingClient().
			Warnf("Error creating http request to to get node list %s, %v", url, err)
		return nodes, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := js.client.Do(request)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nodes, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nodes, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	err = json.Unmarshal(bytes, &nodes)
	if err != nil {
		js.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nodes, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	// get only remote nodes if it is k8s env
	if js.appConfig.IsK8sEnv {
		var remoteNodes []models.Node
		for _, node := range nodes {
			if node.IsRemoteHost {
				remoteNodes = append(remoteNodes, node)
			}
		}
		if len(nodes) != len(remoteNodes) {
			js.service.LoggingClient().Warnf("%d non-remote nodes were removed", len(nodes)-len(remoteNodes))
		}
		return remoteNodes, nil
	}
	return nodes, nil
}

func buildNewDeploymentStatusRecord(
	model ml_model.MLModel,
	node models.Node,
) ml_model.ModelDeploymentStatus {
	deploymentStatus := ml_model.ModelDeploymentStatus{
		MLAlgorithm:          model.MLAlgorithm,
		MLModelConfigName:    model.MLModelConfigName,
		ModelName:            model.ModelName,
		NodeId:               node.NodeId,
		NodeDisplayName:      node.HostName,
		DeploymentStatusCode: ml_model.ReadyToDeploy,
		DeploymentStatus:     ml_model.ReadyToDeploy.String(),
		ModelVersion:         model.ModelVersion,
		PermittedOption:      "deploy",
		IsModelDeprecated:    model.IsModelDeprecated,
	}
	if deploymentStatus.NodeDisplayName == "" {
		deploymentStatus.NodeDisplayName = node.Name
	}
	if model.IsModelDeprecated {
		deploymentStatus.PermittedOption = "none"
		deploymentStatus.DeploymentStatusCode = ml_model.ModelEndOfLife
		deploymentStatus.DeploymentStatus = ml_model.ModelEndOfLife.String()
	}
	return deploymentStatus
}

func (js *TrainingJobService) GetJobsInProgress(
	mlAlgorithm string,
	mlModelConfigName string,
) ([]job.TrainingJobDetails, hedgeErrors.HedgeError) {
	trainingJobs, err := js.dbClient.GetMLTrainingJobsByConfig(mlModelConfigName, "")
	if err != nil {
		return nil, err
	}
	var jobsInProgress []job.TrainingJobDetails
	for _, trainingJob := range trainingJobs {
		if trainingJob.MLAlgorithm == mlAlgorithm &&
			IsTrainingJobInProgress(trainingJob.StatusCode) {
			jobsInProgress = append(jobsInProgress, trainingJob)
		}
	}
	return jobsInProgress, nil
}

func (js *TrainingJobService) GetJobSummary(
	mlModelConfigName string,
	jobStatus string,
) ([]job.TrainingJobSummary, hedgeErrors.HedgeError) {
	var jobList []job.TrainingJobSummary

	errorMessage := fmt.Sprintf(
		"Error getting job summary for model config name %s",
		mlModelConfigName,
	)

	lc := js.service.LoggingClient()
	jobs, err := js.dbClient.GetMLTrainingJobsByConfig(mlModelConfigName, "")

	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return jobList, err
	}
	jobsByte, marshalErr := json.Marshal(jobs)
	if marshalErr != nil {
		lc.Error("%s: marshalling error %v", errorMessage, err)
		return jobList, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	marshalErr = json.Unmarshal(jobsByte, &jobList)
	if marshalErr != nil {
		lc.Errorf("%s: unmarshalling error %v", errorMessage, err)
		return jobList, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	sort.Slice(jobList, func(i, j int) bool {
		return jobList[i].StartTime > jobList[j].StartTime
	})

	if jobStatus != "" {
		filteredJobs := make([]job.TrainingJobSummary, 0)
		for _, singleJob := range jobList {
			if singleJob.Status == jobStatus {
				filteredJobs = append(filteredJobs, singleJob)
			}
		}
		return filteredJobs, nil
	}
	return jobList, nil
}

func (js *TrainingJobService) GetLatestJobSummary(
	mlAlgorithm string,
	mlModelConfigName string,
	jobStatus string,
) (*job.TrainingJobSummary, hedgeErrors.HedgeError) {
	jobs, err := js.GetJobSummary(mlModelConfigName, jobStatus)
	if err != nil {
		return nil, err
	}
	var latestTrainingJob *job.TrainingJobSummary
	for _, jb := range jobs {
		if jb.MLAlgorithm == mlAlgorithm {
			if latestTrainingJob == nil {
				latestTrainingJob = &jb
			} else if jb.StartTime > latestTrainingJob.StartTime {
				latestTrainingJob = &jb
			}
		}
	}
	return latestTrainingJob, nil
}

func (js *TrainingJobService) TerminateLongRunningJobs(
	stopCh <-chan struct{},
	interval time.Duration,
) {
	js.service.LoggingClient().Info("Go routine to Terminate long running jobs")
	for {
		select {
		case <-time.After(interval):
			js.service.LoggingClient().Info("Executing the block for Terminating Long running jobs")
			currentTime := time.Now().Unix()
			jobs, err := js.GetJobs(db2.MLTrainingJob)
			if err != nil {
				js.service.LoggingClient().Error(err.Error())
			}
			filterjobs := js.FilterJobs(jobs, currentTime)
			js.TerminateJobs(filterjobs, currentTime)
			js.lastProcessedTime = currentTime - 21600 // to get jobs related to last 7 hours
		case <-stopCh:
			return // (for tests)
		}
	}
}

func (js *TrainingJobService) FilterJobs(
	jobs []job.TrainingJobDetails,
	currentTime int64,
) []job.TrainingJobDetails {
	var filteredJobs []job.TrainingJobDetails
	for _, job := range jobs {
		if job.StartTime > js.lastProcessedTime {
			filteredJobs = append(filteredJobs, job)
		} else {
			break
		}
	}
	return filteredJobs
}

func (js *TrainingJobService) TerminateJobs(
	jobs []job.TrainingJobDetails,
	currentTime int64,
) []job.TrainingJobDetails {
	var updatedJobs []job.TrainingJobDetails // for test purpose

	for _, job := range jobs {
		if job.EndTime == 0 {
			diff := currentTime - job.StartTime
			if diff >= 21600 { // 6 hours in seconds
				job.StatusCode = 7
				job.EndTime = currentTime
				job.Msg = "Job Terminated Abnormally"
				_, err := js.dbClient.UpdateMLTrainingJob(job)
				if err != nil {
					js.service.LoggingClient().Error(err.Error())
				}
			}
		}
		updatedJobs = append(updatedJobs, job)
	}
	return updatedJobs
}

// AddDefaultMLEventConfig picks default EventConfig from earlier model version if exists, otherwise we create a new one (for Anomaly type only)
func (js *TrainingJobService) AddDefaultMLEventConfig(
	mlModelConfig *config.MLModelConfig,
	modelName string,
) hedgeErrors.HedgeError {

	js.service.LoggingClient().
		Infof("Adding a default MLEvent configuration for trgConfig:%s", mlModelConfig.Name)
	var eventConfigs []config.MLEventConfig
	// Get previous version
	eventConfigs, err := js.dbClient.GetAllMLEventConfigsByConfig(
		mlModelConfig.MLAlgorithm,
		mlModelConfig.Name,
	)
	if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		js.service.LoggingClient().Error(err.Error())
		return err
	}
	algoConfig, err := js.dbClient.GetAlgorithm(mlModelConfig.MLAlgorithm)
	if err != nil {
		js.service.LoggingClient().
			Errorf("Error getting algorithm definition for algo name %s, error %v", mlModelConfig.MLAlgorithm, err)
		return err
	}
	if len(eventConfigs) == 0 && algoConfig != nil && algoConfig.Type == helpers.ANOMALY_ALGO_TYPE {
		// Build default EventConfig
		eventConfig := config.NewDefaultAnomalyEventConfig(
			mlModelConfig.MLAlgorithm,
			mlModelConfig.Name,
		)
		_, err = js.dbClient.AddMLEventConfig(eventConfig)
		if err != nil {
			js.service.LoggingClient().Error(err.Error())
			return err
		}
	} else {
		for _, eventConfig := range eventConfigs {
			_, err = js.dbClient.UpdateMLEventConfig(eventConfig, eventConfig)
			if err != nil {
				js.service.LoggingClient().Error(err.Error())
				return err
			}
		}
	}

	return nil
}

func (js *TrainingJobService) AddMLEventConfig(
	eventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	if eventConfig.StabilizationPeriodByCount <= 0 {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			"stabilization period count needs to be more than 0",
		)
	}

	if eventConfig.MLAlgorithmType == helpers.ANOMALY_ALGO_TYPE {
		// Validate that no event was created for this ML model config (Anomaly only)
		existingEventConfigs, err := js.GetAllMLEventConfigsByConfig(
			eventConfig.MLAlgorithm,
			eventConfig.MlModelConfigName,
		)
		if err != nil {
			errorMsg := fmt.Sprintf(
				"ML event config validation failed: error getting all ML event configs for the ML model %s",
				eventConfig.MlModelConfigName,
			)
			js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errorMsg)
		}
		if len(existingEventConfigs) > 0 {
			js.service.LoggingClient().
				Errorf("ML event config validation failed: only 1 ML event config allowed for algorithm of type Anomaly")
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.MaxLimitExceeded,
				"ML event config validation failed: only 1 ML event config allowed for algorithm of type Anomaly",
			)
		}
	}

	err := js.validateMLEventConfigByAlgoType(eventConfig)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return err
	}

	createdMlEventConfig, err := js.dbClient.AddMLEventConfig(eventConfig)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Error creating ML event config %s for ML model config %s",
			eventConfig.EventName,
			eventConfig.MlModelConfigName,
		)
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errorMsg)
	}

	// Deploy the updated MLEvent config to the nodes
	nodes, err := js.getNodes()
	if err != nil {
		errorMsg := "Error getting nodes for SyncMLEventConfig command"
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return err
	}
	for _, node := range nodes {
		// Build deploymentCommand
		commandForNode := ml_model.ModelDeployCommand{
			MLAlgorithm: createdMlEventConfig.MLAlgorithm,
			TargetNodes: []string{node.NodeId},
			CommandName: "SyncMLEventConfig",
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{SyncCommand: ml_model.CREATE, MLEventConfig: createdMlEventConfig},
			},
		}
		js.service.LoggingClient().
			Debugf("Publishing SyncMLEventConfig command: %v for nodeID: %s", commandForNode, node.NodeId)
		err = js.PublishModelDeploymentCommand(&commandForNode)
		if err != nil {
			js.service.LoggingClient().
				Errorf("Error publishing SyncMLEventConfig command: error:%v", err)
		}
	}

	return nil
}

func (js *TrainingJobService) UpdateMLEventConfig(
	existingMLEvent config.MLEventConfig,
	eventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	if eventConfig.StabilizationPeriodByCount <= 0 {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			"stabilization period count needs to be more than 0",
		)
	}

	err := js.validateMLEventConfigByAlgoType(eventConfig)
	if err != nil {
		js.service.LoggingClient().Error(err.Error())
		return err
	}

	_, err = js.dbClient.UpdateMLEventConfig(existingMLEvent, eventConfig)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Error updating ML event config %s for ML model config %s",
			eventConfig.EventName,
			eventConfig.MlModelConfigName,
		)
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}

	// Deploy the updated MLEvent config to the nodes
	nodes, err := js.getNodes()
	if err != nil {
		errorMsg := "Error getting nodes for SyncMLEventConfig command"
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}
	for _, node := range nodes {
		// Build deploymentCommand
		commandForNode := ml_model.ModelDeployCommand{
			MLAlgorithm:       eventConfig.MLAlgorithm,
			MLModelConfigName: eventConfig.MlModelConfigName,
			TargetNodes:       []string{node.NodeId},
			CommandName:       "SyncMLEventConfig",
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{
					SyncCommand:      ml_model.UPDATE,
					MLEventConfig:    eventConfig,
					OldMLEventConfig: existingMLEvent,
				},
			},
		}
		js.service.LoggingClient().
			Debugf("Publishing SyncMLEventConfig command: %v for nodeID: %s", commandForNode, node.NodeId)
		err = js.PublishModelDeploymentCommand(&commandForNode)
		if err != nil {
			js.service.LoggingClient().
				Errorf("Error publishing SyncMLEventConfig command: error:%v", err)
		}
	}

	return nil
}

func (js *TrainingJobService) DeleteMLEventConfig(
	mlAlgorithmName string,
	mlModelConfigName string,
	mlEventName string,
) hedgeErrors.HedgeError {
	mlEventConfigToDelete, err := js.dbClient.GetMLEventConfigByName(
		mlAlgorithmName,
		mlModelConfigName,
		mlEventName,
	)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Failed to fetch ML event configs for deletion - for %s",
			mlEventName,
		)
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return err
	}

	err = js.dbClient.DeleteMLEventConfigByName(
		mlEventConfigToDelete.MLAlgorithm,
		mlEventConfigToDelete.MlModelConfigName,
		mlEventConfigToDelete.EventName,
	)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Error deleting ML event config %s for ML model config %s",
			mlEventConfigToDelete.EventName,
			mlEventConfigToDelete.MlModelConfigName,
		)
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return err
	}

	// Deploy the updated MLEvent config to the nodes
	nodes, err := js.getNodes()
	if err != nil {
		errorMsg := "Error getting nodes for SyncMLEventConfig command"
		js.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}
	for _, node := range nodes {
		// Build deploymentCommand
		commandForNode := ml_model.ModelDeployCommand{
			MLAlgorithm:       mlEventConfigToDelete.MLAlgorithm,
			MLModelConfigName: mlEventConfigToDelete.MlModelConfigName,
			TargetNodes:       []string{node.NodeId},
			CommandName:       "SyncMLEventConfig",
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{SyncCommand: ml_model.DELETE, MLEventConfig: mlEventConfigToDelete},
			},
		}
		js.service.LoggingClient().
			Debugf("Publishing SyncMLEventConfig command: %v for nodeID: %s", commandForNode, node.NodeId)
		err = js.PublishModelDeploymentCommand(&commandForNode)
		if err != nil {
			js.service.LoggingClient().
				Errorf("Error publishing SyncMLEventConfig command: error:%v", err)
		}
	}

	return nil
}

func (js *TrainingJobService) GetMLEventConfigByName(
	mlAlgorithmName string,
	mlModelConfigName string,
	mlEventName string,
) (config.MLEventConfig, hedgeErrors.HedgeError) {
	return js.dbClient.GetMLEventConfigByName(mlAlgorithmName, mlModelConfigName, mlEventName)
}

func (js *TrainingJobService) GetAllMLEventConfigsByConfig(
	mlAlgorithmName string,
	mlModelConfigName string,
) ([]config.MLEventConfig, hedgeErrors.HedgeError) {
	return js.dbClient.GetAllMLEventConfigsByConfig(mlAlgorithmName, mlModelConfigName)
}

func (js *TrainingJobService) GetMLEventConfig(
	mlAlgorithmName string,
	mlModelConfigName string,
	mlEventName string,
) (interface{}, hedgeErrors.HedgeError) {
	if mlEventName == "all" || mlEventName == "All" {
		mlEventConfigs, err := js.GetAllMLEventConfigsByConfig(mlAlgorithmName, mlModelConfigName)
		if err != nil {
			errorMsg := fmt.Sprintf(
				"Error getting all ML event configs for ML model config %s",
				mlModelConfigName,
			)
			js.service.LoggingClient().Errorf("%s, error: %v", errorMsg, err)
			return nil, err
		}
		return mlEventConfigs, nil
	} else {
		mlEventConfig, err := js.GetMLEventConfigByName(mlAlgorithmName, mlModelConfigName, mlEventName)
		if err != nil {
			errorMsg := fmt.Sprintf("Error getting ML event config %s for ML model config %s", mlEventName, mlModelConfigName)
			js.service.LoggingClient().Errorf("%s, error: %v", errorMsg, err)
			return mlEventConfig, err
		}
		return mlEventConfig, err
	}
}

func (js *TrainingJobService) DeployUndeployMLModel(
	modelDeployCommand *ml_model.ModelDeployCommand,
	algorithm *config.MLAlgorithmDefinition,
) hedgeErrors.HedgeError {
	lc := js.service.LoggingClient()
	currentTime := time.Now().Unix()
	if len(modelDeployCommand.TargetNodes) == 0 {
		lc.Errorf("Target node to deploy/undeploy is mandatory input")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Target node to deploy/undeploy not provided",
		)
	}

	modelDeployCommand.Algorithm = algorithm

	mlmodels, err := js.dbClient.GetLatestModelsByConfig(
		modelDeployCommand.MLAlgorithm,
		modelDeployCommand.MLModelConfigName,
	)
	if err != nil || len(mlmodels) == 0 {
		lc.Errorf("could not get the latest model for model: %s config: %s error: %v",
			modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			"failed getting the latest model for config",
		)
	}
	latestModel := mlmodels[0]
	// Now get the latest MLEventConfig from db
	eventConfigs, err := js.dbClient.GetAllMLEventConfigsByConfig(
		modelDeployCommand.MLAlgorithm,
		modelDeployCommand.MLModelConfigName,
	)
	if err != nil {
		lc.Errorf(
			"Error getting MLEvent configs for model: %s, error:%v",
			modelDeployCommand.MLModelConfigName,
			err,
		)
		return err
	}
	var mlEventConfigsToSync []ml_model.SyncMLEventConfig
	for _, eventConfig := range eventConfigs {
		mlEventConfigsToSync = append(
			mlEventConfigsToSync,
			ml_model.SyncMLEventConfig{
				SyncCommand:      ml_model.UPDATE,
				MLEventConfig:    eventConfig,
				OldMLEventConfig: eventConfig,
			},
		)
	}

	if modelDeployCommand.CommandName == "" {
		modelDeployCommand.CommandName = "Deploy"
	}

	deployStatusCode := ml_model.ReadyToDeploy
	if modelDeployCommand.CommandName == "UnDeploy" {
		deployStatusCode = ml_model.PublishedUnDeployCommand
	}

	commandForNode := ml_model.ModelDeployCommand{
		MLAlgorithm:          modelDeployCommand.MLAlgorithm,
		MLModelConfigName:    modelDeployCommand.MLModelConfigName,
		ModelVersion:         latestModel.ModelVersion,
		ModelName:            latestModel.ModelName,
		CommandName:          modelDeployCommand.CommandName,
		ModelAvailableDate:   latestModel.ModelCreatedTimeSecs,
		MLEventConfigsToSync: mlEventConfigsToSync,
		Algorithm:            modelDeployCommand.Algorithm,
	}

	for _, nodeId := range modelDeployCommand.TargetNodes {
		lc.Infof(
			"publishing deploy/undeploy command for model: %s node: %s",
			commandForNode.ModelName,
			nodeId,
		)

		deploymentStatus := ml_model.ModelDeploymentStatus{
			MLAlgorithm:             modelDeployCommand.MLAlgorithm,
			MLModelConfigName:       modelDeployCommand.MLModelConfigName,
			ModelName:               latestModel.ModelName,
			NodeId:                  nodeId,
			DeploymentStatusCode:    deployStatusCode,
			ModelVersion:            latestModel.ModelVersion,
			ModelDeploymentTimeSecs: currentTime,
		}

		commandForNode.TargetNodes = []string{nodeId}

		err = js.PublishModelDeploymentCommand(&commandForNode)
		if err != nil {
			lc.Errorf("Error publishing model deployment command: error:%v", err)
			if strings.ToLower(commandForNode.CommandName) == "undeploy" {
				deploymentStatus.DeploymentStatusCode = ml_model.ModelUnDeploymentFailed
			} else {
				deploymentStatus.DeploymentStatusCode = ml_model.ModelDeploymentFailed
			}
		} else {
			if strings.ToLower(commandForNode.CommandName) == "undeploy" {
				deploymentStatus.DeploymentStatusCode = ml_model.PublishedUnDeployCommand
			} else {
				deploymentStatus.DeploymentStatusCode = ml_model.PublishedDeployCommand
			}
		}
		deploymentStatus.DeploymentStatus = deploymentStatus.DeploymentStatusCode.String()
		lc.Infof(
			"Status of deploy/undeploy command to node: %s, Status:%s",
			nodeId,
			deploymentStatus.DeploymentStatus,
		)
		err = js.dbClient.UpdateModelDeployment(deploymentStatus)
		if err != nil {
			return err
		}

		commandName := strings.ToLower(commandForNode.CommandName)
		if commandName == "deploy" || commandName == "undeploy" {
			// Write to a channel so we time it and if we don't receive status back from node, we update with failure and timeout
			go js.monitorDeploymentStatus(deploymentStatus, MONITOR_DEPLOYMENT_STATUS_INTERVAL)
		}

	}

	return err
}

// monitorDeploymentStatus monitors deployment on all nodes and marks the deployments as having failed if
// the status has not changed after a timeout.
func (js *TrainingJobService) monitorDeploymentStatus(
	initialStatus ml_model.ModelDeploymentStatus,
	interval time.Duration,
) {

	lc := js.service.LoggingClient()
	for {

		//Note: No need to use a channel here
		time.Sleep(interval)

		lc.Debug(
			"Will check the deployment command status and if no response received, will timeout",
		)
		modelDeploymentsByNode, err := js.dbClient.GetDeploymentsByNode(initialStatus.NodeId)
		if err != nil || len(modelDeploymentsByNode) == 0 {
			lc.Errorf("Failed to get modelDeployment status for %v", initialStatus)
			return
		}

		for _, currentStatus := range modelDeploymentsByNode {

			matchMLAlgorithm := currentStatus.MLAlgorithm == initialStatus.MLAlgorithm
			matchTrainingConfigName := currentStatus.MLModelConfigName == initialStatus.MLModelConfigName
			matchModelVersion := currentStatus.ModelVersion == initialStatus.ModelVersion
			matchDeploymentStatusCode := currentStatus.DeploymentStatusCode == initialStatus.DeploymentStatusCode

			if !(matchMLAlgorithm && matchTrainingConfigName && matchModelVersion) {
				continue
			}

			if !matchDeploymentStatusCode {
				continue
			}

			// If the deployment status has changed, update the status in the database and break the loop
			lc.Errorf(
				"Model Deploy/Undeploy command timed out waiting for status from node, will mark as failure: %v",
				initialStatus,
			)
			if initialStatus.DeploymentStatusCode == ml_model.PublishedDeployCommand {
				initialStatus.DeploymentStatusCode = ml_model.ModelDeploymentFailed
			} else {
				initialStatus.DeploymentStatusCode = ml_model.ModelUnDeploymentFailed
			}
			initialStatus.Message = "Timed out waiting for response from node, check whether ml-edge-agent on node is running"
			js.dbClient.UpdateModelDeployment(initialStatus)
			return

		}

	}
}

func (js *TrainingJobService) PublishModelDeploymentCommand(
	modelDeployCommand *ml_model.ModelDeployCommand,
) hedgeErrors.HedgeError {
	modyReadyNotificationPayload := modelDeployCommand
	modyReadyNotificationPayload.ModelAvailableDate = time.Now().Unix()

	errorMessage := "Failed to publish deploy/undeploy command to edge nodes"

	// Node specific topic name is set by the topicFormatter interface function that is called by MQTTSend
	ok, _ := js.mqttSender.MQTTSend(js.service.BuildContext("gg", "ml"), modelDeployCommand)

	if !ok {
		js.service.LoggingClient().Errorf("%s: command %v", errorMessage, modelDeployCommand)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return nil
}

func (js *TrainingJobService) GetDbClient() redis.MLDbInterface {
	return js.dbClient
}

// Get Latest Candidate models
func buildLatestModelVersionKeys(deployments []ml_model.MLModel) []string {
	algosWithVersion := make([]string, 0)
	var algoKey string
	for _, deployment := range deployments {
		algoKey = deployment.MLModelConfigName + ":v" + fmt.Sprintf("%d", deployment.ModelVersion)
		if !trainingdata.Contains(algosWithVersion, algoKey) {
			algosWithVersion = append(algosWithVersion, algoKey)
		}
	}
	return algosWithVersion
}

func validateUploadedModel(tmpFilePath string) error {
	zipFiles, err := helpers.ReadZipFiles(tmpFilePath)
	if err != nil {
		return err
	}

	modelExtensions := []string{".pickle", ".h5", ".pkl"}

	isConfigFound := false
	isModelFound := false

	// Iterate through the files in the ZIP archive
	for _, file := range zipFiles {
		ext := filepath.Ext(file.Path)
		name := filepath.Base(file.Path)
		if slices.Contains(modelExtensions, ext) {
			isModelFound = true
		}
		if name == "config.json" {
			isConfigFound = true
		}
	}
	if !isConfigFound {
		return errors.New("Uploaded model zip file doesn't contain config.json")
	}
	if !isModelFound {
		return errors.New("Uploaded model zip file doesn't contain a trained model")
	}
	return nil
}

func (js *TrainingJobService) validateMLEventConfigByAlgoType(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	// Common validations:
	err := js.applyCommonValidations(mlEventConfig)
	if err != nil {
		return err
	}
	switch mlEventConfig.MLAlgorithmType {
	case helpers.ANOMALY_ALGO_TYPE:
		err = js.applyAnomalyAlgoTypeValidations(mlEventConfig)
		if err != nil {
			return err
		}
	case helpers.CLASSIFICATION_ALGO_TYPE:
		err = js.applyClassificationAlgoTypeValidations(mlEventConfig)
		if err != nil {
			return err
		}
	case helpers.TIMESERIES_ALGO_TYPE:
		err = js.applyTimeseriesAlgoTypeValidations(mlEventConfig)
		if err != nil {
			return err
		}
	case helpers.REGRESSION_ALGO_TYPE:
		err = js.applyRegressionAlgoTypeValidations(mlEventConfig)
		if err != nil {
			return err
		}
	default:
		errMsg := fmt.Sprintf(
			"ML event config validation failed: unknown algorithm type: %s",
			mlEventConfig.MLAlgorithmType,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}

	return nil
}

func (js *TrainingJobService) applyCommonValidations(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	severityCounts := map[string]int{
		models.SEVERITY_CRITICAL: 0,
		models.SEVERITY_MAJOR:    0,
		models.SEVERITY_MINOR:    0,
	}
	if len(mlEventConfig.Conditions) == 0 {
		errMsg := "ML event config validation failed: invalid amount of conditions provided - 0 (expected at least 1 condition)"

		js.service.LoggingClient().
			Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	for _, condition := range mlEventConfig.Conditions {
		severityCounts[condition.SeverityLevel]++
		if severityCounts[condition.SeverityLevel] > 1 {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: severity level %s appears more than once",
				condition.SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		// Validate thresholds in the condition
		if len(condition.ThresholdsDefinitions) == 0 {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: condition for severity level %s must contain at least one threshold, but none is provided",
				condition.SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}

		threshold := condition.ThresholdsDefinitions[0]
		// Validate threshold based on the operator
		if threshold.Operator != config.BETWEEN_OPERATOR &&
			threshold.Operator != config.GREATER_THAN_OPERATOR &&
			threshold.Operator != config.LESS_THAN_OPERATOR &&
			threshold.Operator != config.EQUAL_TO_OPERATOR {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: unknown operator %s in severity level %s",
				threshold.Operator,
				condition.SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		if threshold.Operator == config.BETWEEN_OPERATOR {
			// BETWEEN operator must have lower and upper thresholds, upper threshold must be greater than lower threshold
			if threshold.LowerThreshold == 0 && threshold.UpperThreshold == 0 {
				errMsg := fmt.Sprintf(
					"ML event config validation failed: BETWEEN operator requires both lower and upper thresholds in severity level %s",
					condition.SeverityLevel,
				)
				js.service.LoggingClient().Errorf(errMsg)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
			}
			if threshold.UpperThreshold <= threshold.LowerThreshold {
				errMsg := fmt.Sprintf(
					"ML event config validation failed: invalid BETWEEN thresholds: UpperThreshold (%f) must be greater than LowerThreshold (%f) for severity level %s",
					threshold.UpperThreshold,
					threshold.LowerThreshold,
					condition.SeverityLevel,
				)
				js.service.LoggingClient().Errorf(errMsg)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
			}
		}
	}

	return nil
}

func (js *TrainingJobService) applyAnomalyAlgoTypeValidations(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	// 1. Validate number of conditions: no more than 3 conditions allowed per event
	amountOfConditions := len(mlEventConfig.Conditions)
	if amountOfConditions > 3 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of conditions provided - %d (maximum allowed for Anomaly type - 3)",
			amountOfConditions,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// Map to track ranges for overlap validation
	rangesBySeverity := make(map[string][][2]float64)
	// 2. Validate thresholds per condition
	for _, condition := range mlEventConfig.Conditions {
		if len(condition.ThresholdsDefinitions) > 1 {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: invalid amount of thresholds provided per condition - %d (maximum allowed for Anomaly type - 1)",
				len(condition.ThresholdsDefinitions),
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		threshold := condition.ThresholdsDefinitions[0]
		thresholdValue, err := utils.ToFloat64(threshold.ThresholdValue)
		if (err == nil && thresholdValue > MAX_ANOMALY_THRESHOLD_VALUE) ||
			threshold.UpperThreshold > MAX_ANOMALY_THRESHOLD_VALUE {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: threshold value exceeds the maximum allowed value of 65535 in severity level - %s",
				condition.SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		// 2.1 Apply operator-specific ranges
		var lowerBound, upperBound float64
		switch threshold.Operator {
		case config.LESS_THAN_OPERATOR:
			lowerBound = 0
			upperBound = thresholdValue
		case config.GREATER_THAN_OPERATOR:
			lowerBound = thresholdValue
			upperBound = MAX_ANOMALY_THRESHOLD_VALUE
		case config.BETWEEN_OPERATOR:
			lowerBound = threshold.LowerThreshold
			upperBound = threshold.UpperThreshold
		default:
			errMsg := fmt.Sprintf(
				"ML event config validation failed: unknown operator %s in severity level - %s",
				threshold.Operator,
				condition.SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		// 2.2 Ensure no overlap between ranges
		for severity, ranges := range rangesBySeverity {
			for _, r := range ranges {
				if !(upperBound <= r[0] || lowerBound >= r[1]) {
					errMsg := fmt.Sprintf(
						"ML event config validation failed: range [%.2f, %.2f] for severity %s overlaps with range [%.2f, %.2f] for severity %s",
						lowerBound,
						upperBound,
						condition.SeverityLevel,
						r[0],
						r[1],
						severity,
					)
					js.service.LoggingClient().Errorf(errMsg)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
				}
			}
		}
		// Add the validated range to the map
		rangesBySeverity[condition.SeverityLevel] = append(
			rangesBySeverity[condition.SeverityLevel],
			[2]float64{lowerBound, upperBound},
		)
	}

	return nil
}

func (js *TrainingJobService) applyClassificationAlgoTypeValidations(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	// 1. Validate number of conditions
	amountOfConditions := len(mlEventConfig.Conditions)
	if amountOfConditions > 1 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of conditions provided - %d (maximum allowed for Classification type - 1)",
			amountOfConditions,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// 2. Validate number of thresholds per condition: only 2 thresholds allowed per condition (1 - class name, 2 - confidence)
	amountOfThresholdsPerCondition := len(mlEventConfig.Conditions[0].ThresholdsDefinitions)
	if amountOfThresholdsPerCondition != 2 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of thresholds provided per condition - %d (expected for Classification type - 2)",
			amountOfThresholdsPerCondition,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// 3. Validate that one of the thresholds has a ThresholdValue of type string
	isStringTypeFound := false
	for _, threshold := range mlEventConfig.Conditions[0].ThresholdsDefinitions {
		if _, ok := threshold.ThresholdValue.(string); ok {
			isStringTypeFound = true
			break
		}
	}
	if !isStringTypeFound {
		errMsg := "ML event config validation failed: none of the thresholds has a ThresholdValue of type string (class name)"
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}

	return nil
}

func (js *TrainingJobService) applyTimeseriesAlgoTypeValidations(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	// 1. Validate number of conditions
	amountOfConditions := len(mlEventConfig.Conditions)
	if amountOfConditions > 1 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of conditions provided - %d (maximum allowed for Timeseries type - 1)",
			amountOfConditions,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// 2. Validate features names (labels) in the thresholds
	labelCounts := make(map[string]bool)
	for _, threshold := range mlEventConfig.Conditions[0].ThresholdsDefinitions {
		if threshold.Label == "" {
			errMsg :=
				"ML event config validation failed: feature name provided for prediction (label) is empty"
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		// Validate the feature name (label) provided for prediction matches the pattern (<P/profile name>#<F/feature name>)
		labelPattern := `^[a-zA-Z0-9_-]+#[a-zA-Z0-9_-]+$`
		regex := regexp.MustCompile(labelPattern)
		if !regex.MatchString(threshold.Label) {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: threshold.Label '%s' does not match the required pattern: '<P/profile name>#<F/feature name>'",
				threshold.Label,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		// Check for duplicate feature names (labels) in thresholds
		if labelCounts[threshold.Label] {
			errMsg := fmt.Sprintf(
				"ML event config validation failed: duplicated feature name %s in thresholds for severity level %s",
				threshold.Label,
				mlEventConfig.Conditions[0].SeverityLevel,
			)
			js.service.LoggingClient().Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
		}
		labelCounts[threshold.Label] = true
	}
	// 3. Validate StabilizationPeriodByCount
	if mlEventConfig.StabilizationPeriodByCount != 1 {
		errMsg := "ML event config validation failed: stabilizationPeriodByCount must be 1 for Timeseries"
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}

	return nil
}

func (js *TrainingJobService) applyRegressionAlgoTypeValidations(
	mlEventConfig config.MLEventConfig,
) hedgeErrors.HedgeError {
	// 1. Validate number of conditions
	amountOfConditions := len(mlEventConfig.Conditions)
	if amountOfConditions > 1 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of conditions provided - %d (maximum allowed for Regression type - 1)",
			amountOfConditions,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// 2. Validate number of thresholds per condition: only 1 threshold allowed per condition
	amountOfThresholdsPerCondition := len(mlEventConfig.Conditions[0].ThresholdsDefinitions)
	if amountOfThresholdsPerCondition > 1 {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: invalid amount of thresholds provided per condition - %d (maximum allowed for Regression type - 1)",
			amountOfThresholdsPerCondition,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// 3. Validate label (feature name for regression prediction)
	if mlEventConfig.Conditions[0].ThresholdsDefinitions[0].Label == "" {
		errMsg := "ML event config validation failed: prediction class name provided (label) is empty"
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}
	// Validate the feature name (label) provided for prediction matches the pattern (<P/profile name>#<F/feature name>)
	labelPattern := `^[a-zA-Z0-9_-]+#[a-zA-Z0-9_-]+$`
	regex := regexp.MustCompile(labelPattern)
	if !regex.MatchString(mlEventConfig.Conditions[0].ThresholdsDefinitions[0].Label) {
		errMsg := fmt.Sprintf(
			"ML event config validation failed: threshold.Label '%s' does not match the required pattern: '<P/profile name>#<F/feature name>'",
			mlEventConfig.Conditions[0].ThresholdsDefinitions[0].Label,
		)
		js.service.LoggingClient().Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}

	return nil
}
