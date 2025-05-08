package hedge

import (
	"hedge/common/client"
	mqttConfig "hedge/common/config"
	hedgeErrors "hedge/common/errors"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
	"path/filepath"
	"sync"

	//"hedge/edge-ml-service/internal/training"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
)

type HedgeTrainingProvider struct {
	appConfig                   config.MLMgmtConfig
	service                     interfaces.ApplicationService
	mlStorage                   helpers.MLStorageInterface
	HedgeTrainingProviderConfig HedgeTrainingProviderConfig
}

var trainingJobsStatusByJob sync.Map

func BuildJobConfig(jobSubmissionDetails job.JobSubmissionDetails, mlAlgorithmDefinition *config.MLAlgorithmDefinition, mlModelConfig *config.MLModelConfig, service interfaces.ApplicationService) (*job.TrainingJobDetails, hedgeErrors.HedgeError) {

	trainingImagePath := mlAlgorithmDefinition.TrainerImagePath
	jobBaseDir, err1 := service.GetAppSetting("JobDir")
	if err1 != nil {
		service.LoggingClient().Errorf("error reading JobDir from config")
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			fmt.Sprintf("error reading JobDir from config: %s", err1.Error()))
	}

	trainingJobFileName := filepath.Join(jobSubmissionDetails.MLAlgorithm, jobSubmissionDetails.MLModelConfigName, helpers.TRAINING_ZIP_FILENAME)
	localModelFileName := filepath.Join(jobSubmissionDetails.MLAlgorithm, jobSubmissionDetails.MLModelConfigName, helpers.MODEL_ZIP_FILENAME)
	trainingTriggerTopic, err1 := service.GetAppSetting("TrainingTriggerTopic")
	if err1 != nil {
		service.LoggingClient().Errorf("error reading TrainingTriggerTopic from config")
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			fmt.Sprintf("error reading TrainingTriggerTopic from config: %s", err1.Error()))
	}

	hedgeTrgJobConfig := job.NewTrainingJob(jobSubmissionDetails.Name, mlModelConfig)
	hedgeTrgJobConfig.JobServiceConfig = HedgeTrainingProviderConfig{
		TrainingImagePath:          trainingImagePath,
		JobDir:                     jobBaseDir,
		trainingDataFileName:       trainingJobFileName,
		modelFileName:              localModelFileName,
		TopicNameToTriggerTraining: trainingTriggerTopic,
	}
	// New job status
	jobStatus := job.HedgeJobStatus{JobName: jobSubmissionDetails.Name, Status: "NEW"}
	trainingJobsStatusByJob.Store(jobSubmissionDetails.Name, jobStatus)
	return hedgeTrgJobConfig, nil
}

// Handles training status updates for local hedge trainer in ml-mgmt pipeline
func HandleTrainingStatusUpdates(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {

	jsonBytes, err := util.CoerceType(data)
	if err != nil {
		ctx.LoggingClient().Errorf("error unmarshaling data received to mqtt status topics, error: %s", err.Error())
		return false, data
	}
	var jobStatus job.HedgeJobStatus
	err = json.Unmarshal(jsonBytes, &jobStatus)
	if err != nil || (err == nil && jobStatus.JobName == "") {
		// It means it is not HedgeJobStatus
		ctx.LoggingClient().Infof("data received on status topic is deployment status, so forwarding to next step in pipeline")
		return true, jsonBytes
	}
	trainingJobsStatusByJob.Store(jobStatus.JobName, jobStatus)

	ctx.LoggingClient().Infof("jobStatus received from hedge training provider: %v", jobStatus)
	return false, nil
}

// Data structure to hold HedgeTrainingProvider specific configuration details
type HedgeTrainingProviderConfig struct {
	JobDir string
	// trainingDataFile is relative path wrt JobDir
	trainingDataFileName string
	// local trained model zip file name after the training is completed, relative to jobDir
	modelFileName              string
	TrainingImagePath          string
	TopicNameToTriggerTraining string
}

func NewHedgeTrainingProvider(appConfig config.MLMgmtConfig, service interfaces.ApplicationService, jobDetails *job.TrainingJobDetails) *HedgeTrainingProvider {
	hedgeTrgProvider := new(HedgeTrainingProvider)
	hedgeTrgProvider.appConfig = appConfig
	hedgeTrgProvider.service = service
	// Instance specific parameters now
	hedgeTrgProvider.HedgeTrainingProviderConfig = jobDetails.JobServiceConfig.(HedgeTrainingProviderConfig)
	//jobDetails.MLAlgorithm
	// Source training file directory here
	hedgeTrgProvider.mlStorage = helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		jobDetails.MLAlgorithm,
		jobDetails.MLModelConfigName,
		service.LoggingClient(),
	)

	return hedgeTrgProvider
}

// Upload the training data file to jobDir specific to MLModelConfig
func (h *HedgeTrainingProvider) UploadFile(remoteFile string, localFile string) error {
	err := h.mlStorage.CompressFolder(h.mlStorage.GetLocalTrainingDataBaseDir(), h.mlStorage.GetTrainingInputZipFile())
	if err != nil {
		return err
	}
	trainingDataFileName := h.mlStorage.GetTrainingInputZipFile()

	targetFileName := filepath.Join(h.HedgeTrainingProviderConfig.JobDir, h.HedgeTrainingProviderConfig.trainingDataFileName)
	err = helpers.CopyFile(trainingDataFileName, targetFileName)
	if err != nil {
		h.service.LoggingClient().Errorf("Failed to copy training data file %s to %s. Error: %v", trainingDataFileName, h.HedgeTrainingProviderConfig.JobDir, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	// Change the permission of the job directory so python code can read/write in here
	err = helpers.ChangeFileOwnership(h.HedgeTrainingProviderConfig.JobDir, h.HedgeTrainingProviderConfig.trainingDataFileName, 2002, 2001)
	if err != nil {
		h.service.LoggingClient().Errorf("Failed to change ownership of file %s, Error: %v", trainingDataFileName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}

	return nil
}

func (h *HedgeTrainingProvider) SubmitTrainingJob(jobConfig *job.TrainingJobDetails) error {

	imagePath := jobConfig.JobServiceConfig.(HedgeTrainingProviderConfig).TrainingImagePath
	localJob := job.HedgeTrainJob{
		JobName:   jobConfig.Name,
		ImagePath: imagePath,
		DataFile:  jobConfig.JobServiceConfig.(HedgeTrainingProviderConfig).trainingDataFileName,
	}
	// Publish to MQTT
	trainingTriggerTopic, err := h.service.GetAppSetting("TrainingTriggerTopic")
	if err != nil {
		h.service.LoggingClient().Errorf("error reading TrainingTriggerTopic from config")
		return nil
	}
	config, err := mqttConfig.BuildMQTTSecretConfig(
		h.service,
		trainingTriggerTopic,
		client.HedgeMLManagementServiceName+jobConfig.MLModelConfigName,
	)
	if err != nil {
		h.service.LoggingClient().Errorf("Error Building MQTT client: Error: %v", err)
		return nil
	}

	MQTTSender := transforms.NewMQTTSecretSender(config, false)
	ok, _ := MQTTSender.MQTTSend(h.service.BuildContext("train", "ml"), localJob)
	if ok {
		return nil
	}
	h.service.LoggingClient().Errorf("error publishing training command to MQTT, mlModelConfig: %s", jobConfig.MLModelConfigName)
	return errors.New("error publishing training command to MQTT")
}

// Update the job Status in jobDetails after the job was submitted by querying the training provider
func (h *HedgeTrainingProvider) GetTrainingJobStatus(jobDtls *job.TrainingJobDetails) error {
	jobStatus, ok := trainingJobsStatusByJob.Load(jobDtls.Name)
	if !ok {
		return errors.New(fmt.Sprintf("jobStatus not found for jobName: %s", jobDtls.Name))
	}
	statusJobContainer, _ := jobStatus.(job.HedgeJobStatus)

	if statusJobContainer.Status == "SUCCESSFUL" {
		jobDtls.StatusCode = job.TrainingCompleted
		jobDtls.Msg = ""
		trainingJobsStatusByJob.Delete(jobDtls.Name)
	} else if statusJobContainer.Status == "FAILED" {
		jobDtls.StatusCode = job.Failed
		jobDtls.Msg = statusJobContainer.Message
		trainingJobsStatusByJob.Delete(jobDtls.Name)
	} else {
		jobDtls.StatusCode = 3
		jobDtls.Msg = "Training In Progress"
	}

	return nil
}

// Copy the model to the right place in ml-management
func (h *HedgeTrainingProvider) DownloadModel(localModelDirectory string, modelFileName string) error {

	sourceModelFileName := filepath.Join(h.HedgeTrainingProviderConfig.JobDir, h.HedgeTrainingProviderConfig.modelFileName)
	targetModelFile := h.mlStorage.GetModelLocalZipFileName()
	h.service.LoggingClient().Infof("About to copy the trained model from: %s to %s", sourceModelFileName, targetModelFile)
	err := helpers.CopyFile(sourceModelFileName, targetModelFile)
	if err != nil {
		h.service.LoggingClient().Errorf("Failed to copy trained model file from %s to %s. Error: %v", sourceModelFileName, targetModelFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}

	h.service.LoggingClient().Debugf("Successfully copied model file to: %s", targetModelFile)
	return nil
}
