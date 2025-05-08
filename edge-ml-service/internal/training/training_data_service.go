/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	hedgeErrors "hedge/common/errors"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"
	trainingdata "hedge/edge-ml-service/pkg/training-data"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Batch size to pull training data from Victoria Matrix or ADE metric store
const BATCH_SIZE int64 = 5
const CONFIG_FILE = "config.json"

// for tests
var ExecCommandFunc func(string, ...string) *exec.Cmd = exec.Command

type TrainingDataServiceInterface interface {
	CheckIfJobExists(jobName string) hedgeErrors.HedgeError
	CreateTrainingDataExport(mlModelConfig *config.MLModelConfig, generateConfigOnly bool)
	//CreateTrainingData(mlModelConfig *config.MLModelConfig, jobSubmissionDetails *job.JobSubmissionDetails) (string, hedgeErrors.HedgeError)
	CreateTrainingDataLocal(mlModelConfig *config.MLModelConfig, jobSubmissionDetails *job.JobSubmissionDetails) (string, hedgeErrors.HedgeError)
	CreateTrainingSampleAsync(mlModelConfig *config.MLModelConfig, dataSampleSize int)
	//CreateTrainingSample(mlModelConfig *config.MLModelConfig, dataSampleSize int) ([]string, error)
	NewDataCollectionProcessor(mlModelConfig *config.MLModelConfig, service interfaces.ApplicationService, appConfig *config.MLMgmtConfig) *trainingdata.DataCollector
	GetAppConfig() *config.MLMgmtConfig
	GetDbClient() redis.MLDbInterface
	GetDataStoreProvider() service.DataStoreProvider
	GetService() interfaces.ApplicationService
	SaveUploadedTrainingData(mlModelConfig *config.MLModelConfig, uploadedFile multipart.File) hedgeErrors.HedgeError
	ValidateUploadedData(mlModelConfig *config.MLModelConfig, mlAlgorithmDefinition *config.MLAlgorithmDefinition) (string, hedgeErrors.HedgeError)
	CleanupStatuses(mlModelConfigs []config.MLModelConfig)
	SetTrainingDataExportDeprecated(mlModelConfig *config.MLModelConfig) hedgeErrors.HedgeError
}

type ExportDataContext struct {
	isCancelling bool
}

type TrainingDataService struct {
	service           interfaces.ApplicationService
	appConfig         *config.MLMgmtConfig
	dbClient          redis.MLDbInterface
	dataStoreProvider service.DataStoreProvider
	// Below method is provided so unit test cases can override the datacollector with a mock
	dataCollector            trainingdata.DataCollectorInterface
	cmdRunner                service.CommandRunnerInterface
	statusMutex              sync.Mutex
	connectionHandler        interface{}
	exportCtx                map[string]*ExportDataContext
	dataCollectorExecutionId string
}

func NewTrainingDataService(service interfaces.ApplicationService, appConfig *config.MLMgmtConfig, dbClient redis.MLDbInterface, dataSourceProvider service.DataStoreProvider, cmdRunner service.CommandRunnerInterface, connectionHandler interface{}) TrainingDataServiceInterface {
	trainingDataSvc := TrainingDataService{
		service:           service,
		appConfig:         appConfig,
		dbClient:          dbClient,
		dataStoreProvider: dataSourceProvider,
		cmdRunner:         cmdRunner,
		connectionHandler: connectionHandler,
		exportCtx:         make(map[string]*ExportDataContext),
	}
	return &trainingDataSvc
}

func (trgSvc *TrainingDataService) overrideJob(jobToCheck job.TrainingJobDetails) bool {
	return jobToCheck.StatusCode == job.New || jobToCheck.StatusCode == job.TrainingDataCollected || jobToCheck.StatusCode == job.Failed || jobToCheck.StatusCode == job.Cancelled
}

func (trgSvc *TrainingDataService) CleanupStatuses(mlModelConfigs []config.MLModelConfig) {
	for _, modelConfig := range mlModelConfigs {
		exportStatusPath := filepath.Join(trgSvc.appConfig.BaseTrainingDataLocalDir, modelConfig.MLAlgorithm, modelConfig.Name, "data_export", "status.json")
		trgSvc.cleanupStatusFile(exportStatusPath)
		sampleStatusPath := filepath.Join(trgSvc.appConfig.BaseTrainingDataLocalDir, modelConfig.MLAlgorithm, modelConfig.Name, "sample", "status.json")
		trgSvc.cleanupStatusFile(sampleStatusPath)
	}
}

func (trgSvc *TrainingDataService) cleanupStatusFile(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return
	}
	status, err := trainingdata.ReadStatus(filePath)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to cleanup file %s. It will be removed. Failed to read status: %v", filePath, err)
		err = os.Remove(filePath)
		if err != nil {
			trgSvc.service.LoggingClient().Errorf("Failed to remove file %s: %v", filePath, err)
		}
		return
	}
	if status.Status != trainingdata.StatusInProgress {
		return
	}
	status.Status = trainingdata.StatusFailed
	status.ErrorMessage = "Deprecated since the application has been restarted"
	err = trainingdata.UpdateStatus(filePath, status)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to cleanup file %s. It will be removed. Failed to update status: %v", filePath, err)
		err = os.Remove(filePath)
		if err != nil {
			trgSvc.service.LoggingClient().Errorf("Failed to remove file %s: %v", filePath, err)
		}
		return
	}
}

func (trgSvc *TrainingDataService) CheckIfJobExists(jobName string) hedgeErrors.HedgeError {
	existingJob, err := trgSvc.dbClient.GetMLTrainingJob(jobName)
	if err == nil && trgSvc.overrideJob(existingJob) {
		//ok to overwrite since it was in error
		return nil // Update so we can re-use same
	}

	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return nil
		}

		return err
	}

	if existingJob.Name != "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Job %s already exists", jobName))
	}

	return nil
}

func (trgSvc *TrainingDataService) initStatusFile(statusFilePath string) bool {
	trgSvc.statusMutex.Lock()
	defer trgSvc.statusMutex.Unlock()

	status, err := trainingdata.ReadStatus(statusFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			trgSvc.service.LoggingClient().Errorf("Failed to read status. File %s will be overritten. Error: %v", statusFilePath, err)
		}
	} else {
		if status.Status == trainingdata.StatusInProgress {
			trgSvc.service.LoggingClient().Warnf("Status already in progress: %s", statusFilePath)
			return false
		}
	}
	if !trgSvc.updateStatusResponse(trainingdata.StatusInProgress, "", statusFilePath) {
		return false
	}
	return true
}

func (trgSvc *TrainingDataService) updateStatusResponse(status string, errMsg string, statusFilePath string) bool {
	statusResponse := trainingdata.StatusResponse{
		Status:       status,
		ErrorMessage: errMsg,
	}
	err := trainingdata.UpdateStatus(statusFilePath, &statusResponse)
	if err != nil {
		trgSvc.service.LoggingClient().Error("Failed to update status file %s to status %v. Error: %v", statusFilePath, statusResponse, err)
		return false
	}
	return true
}

func (trgSvc *TrainingDataService) getExportDataCtxKey(mlModelConfig *config.MLModelConfig) string {
	return mlModelConfig.MLAlgorithm + "_" + mlModelConfig.Name
}

func (trgSvc *TrainingDataService) initExportStatusFile(statusFilePath string, mlModelConfig *config.MLModelConfig) *ExportDataContext {
	trgSvc.statusMutex.Lock()
	defer trgSvc.statusMutex.Unlock()
	existingCtx, exists := trgSvc.exportCtx[trgSvc.getExportDataCtxKey(mlModelConfig)]
	if exists {
		if existingCtx.isCancelling {
			trgSvc.service.LoggingClient().Warnf("Data export cancellation is in progress for mlAlgorithm: %s, mlModelConfig: %s", mlModelConfig.MLAlgorithm, mlModelConfig.Name)
		} else {
			// if ctx exists, then the status must be IN_PROGRESS
			trgSvc.service.LoggingClient().Warnf("Status already in progress")
		}
		return nil
	}
	if !trgSvc.updateStatusResponse(trainingdata.StatusInProgress, "", statusFilePath) {
		return nil
	}
	ctx := ExportDataContext{
		isCancelling: false,
	}
	trgSvc.exportCtx[trgSvc.getExportDataCtxKey(mlModelConfig)] = &ctx
	return &ctx
}

func (trgSvc *TrainingDataService) SetTrainingDataExportDeprecated(mlModelConfig *config.MLModelConfig) hedgeErrors.HedgeError {
	mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())
	basePath := mlStorage.GetLocalTrainingDataBaseDir()
	exportBasePath := strings.TrimSuffix(basePath, "data") + "data_export"
	statusFilePath := filepath.Join(exportBasePath, "status.json")

	trgSvc.statusMutex.Lock()
	defer trgSvc.statusMutex.Unlock()

	errMsg := "Deprecate training data export failed"

	status, err := trainingdata.ReadStatus(statusFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "")
		}
		trgSvc.service.LoggingClient().Errorf("Failed to read status file: %s. Error: %v", statusFilePath, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	switch status.Status {
	case trainingdata.StatusCompleted:
		if !trgSvc.updateStatusResponse(trainingdata.StatusDeprecated, "", statusFilePath) {
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
		}
		return nil
	case trainingdata.StatusInProgress:
		ctx := trgSvc.exportCtx[trgSvc.getExportDataCtxKey(mlModelConfig)]
		if ctx == nil {
			trgSvc.service.LoggingClient().Errorf("Failed to get ctx for model config %s", mlModelConfig.Name)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
		}
		if ctx.isCancelling {
			trgSvc.service.LoggingClient().Warnf("Already cancelling data export for mlAlgorithm: %s, mlModelConfig: %s", mlModelConfig.MLAlgorithm, mlModelConfig.Name)
			return nil
		}
		ctx.isCancelling = true
		return nil
	default:
		return nil
	}
}

func (trgSvc *TrainingDataService) finalizeDataExport(mlModelConfig *config.MLModelConfig, ctx *ExportDataContext, statusFilePath string, status string, errorMsg string) {
	trgSvc.statusMutex.Lock()
	defer trgSvc.statusMutex.Unlock()
	delete(trgSvc.exportCtx, trgSvc.getExportDataCtxKey(mlModelConfig))
	if ctx.isCancelling {
		_ = trgSvc.updateStatusResponse(trainingdata.StatusDeprecated, "", statusFilePath)
	} else {
		_ = trgSvc.updateStatusResponse(status, errorMsg, statusFilePath)
	}
}

// CreateTrainingDataExport Downloads the training data in a csv file and updates the status of download in the database
/*
func (trgSvc *TrainingDataService) CreateTrainingDataExport(mlModelConfig *config.MLModelConfig, generateConfigOnly bool) {
	if trgSvc.appConfig.LocalDataCollection {
		trgSvc.createTrainingDataExportLocal(mlModelConfig, generateConfigOnly)
		return
	}

	trgSvc.service.LoggingClient().Info("Creating training data export")
	mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())

	statusFileName := "status.json"

	basePath := mlStorage.GetLocalTrainingDataBaseDir()
	exportBasePath := strings.TrimSuffix(basePath, "data") + "data_export"

	statusFilePath := filepath.Join(exportBasePath, statusFileName)
	zipFilePath := filepath.Join(exportBasePath, "training_input.zip")

	ctx := trgSvc.initExportStatusFile(statusFilePath, mlModelConfig)
	if ctx == nil {
		return
	}

	if !generateConfigOnly {
		fileId, hedgeErr := trgSvc.collectTrainingData(mlModelConfig, mlStorage, exportBasePath, job.ExportDataCollecting)
		if hedgeErr != nil {
			trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, hedgeErr.Message())
			return
		}

		err := trgSvc.downloadFile(fileId, zipFilePath)
		if err != nil {
			trgSvc.service.LoggingClient().Error(err.Error())
			trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, err.Error())
			return
		}
	} else {
		zipBaseDir := strings.TrimPrefix(mlStorage.GetLocalTrainingDataBaseDir(), mlStorage.GetBaseLocalDirectory()+string(filepath.Separator))
		configFilePath := filepath.Join(exportBasePath, CONFIG_FILE)

		var filesToZip []helpers.ZipFileInfo

		filesToZip = append(filesToZip,
			helpers.ZipFileInfo{
				FilePath:      configFilePath,
				FilePathInZip: filepath.Join(zipBaseDir, CONFIG_FILE),
			})

		err := mlStorage.CompressFiles(filesToZip, zipFilePath)
		if err != nil {
			trgSvc.service.LoggingClient().Errorf(err.Error())
			trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, err.Error())
			return
		}
	}
	trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusCompleted, "")
}
*/

func (trgSvc *TrainingDataService) CreateTrainingDataExport(mlModelConfig *config.MLModelConfig, generateConfigOnly bool) {
	dataCollector, err := trgSvc.GetDataCollector(mlModelConfig)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("data collector could not be created, check trainingDataConfig. Error: %s", err.Error())
		return
	}

	statusFileName := "status.json"
	dataFileName := mlModelConfig.Name + ".csv"
	configFileName := CONFIG_FILE

	basePath := dataCollector.GetMlStorage().GetLocalTrainingDataBaseDir()
	exportBasePath := strings.TrimSuffix(basePath, "data") + "data_export"

	statusFilePath := filepath.Join(exportBasePath, statusFileName)
	dataFilePath := filepath.Join(exportBasePath, dataFileName)
	configFilePath := filepath.Join(exportBasePath, configFileName)

	ctx := trgSvc.initExportStatusFile(statusFilePath, mlModelConfig)
	if ctx == nil {
		return
	}

	trgSvc.service.LoggingClient().Info("Creating training data export local")

	dataCollectorError := dataCollector.GenerateConfig(configFilePath)
	if dataCollectorError != nil {
		trgSvc.service.LoggingClient().Warnf(dataCollectorError.Error())
		trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, dataCollectorError.Error())
		return
	}

	if !generateConfigOnly {
		err := dataCollector.ExecuteWithFileNames(dataFilePath)
		if err != nil {
			trgSvc.service.LoggingClient().Warn(err.Error())
			trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, err.Error())
			return
		}
	}

	zipBaseDir := strings.TrimPrefix(dataCollector.GetMlStorage().GetLocalTrainingDataBaseDir(), dataCollector.GetMlStorage().GetBaseLocalDirectory()+string(filepath.Separator))

	var filesToZip []helpers.ZipFileInfo

	if !generateConfigOnly {
		filesToZip = append(filesToZip,
			helpers.ZipFileInfo{
				FilePath:      dataFilePath,
				FilePathInZip: filepath.Join(zipBaseDir, mlModelConfig.Name+".csv"),
			})
	}

	filesToZip = append(filesToZip,
		helpers.ZipFileInfo{
			FilePath:      configFilePath,
			FilePathInZip: filepath.Join(zipBaseDir, CONFIG_FILE),
		})

	zipFilePath := filepath.Join(exportBasePath, "training_input.zip")
	dataCollectorError = dataCollector.GetMlStorage().CompressFiles(filesToZip, zipFilePath)
	if dataCollectorError != nil {
		trgSvc.service.LoggingClient().Warnf(dataCollectorError.Error())
		trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusFailed, dataCollectorError.Error())
		return
	}
	trgSvc.finalizeDataExport(mlModelConfig, ctx, statusFilePath, trainingdata.StatusCompleted, "")
}

/*
// This was to createTrg data from ADE data provider

	func (trgSvc *TrainingDataService) CreateTrainingData(mlModelConfig *config.MLModelConfig, jobSubmissionDetails *job.JobSubmissionDetails) (string, hedgeErrors.HedgeError) {
		trgJob, err := trgSvc.initCreateTrainingData(mlModelConfig, jobSubmissionDetails)
		if err != nil {
			return "", err
		}
		errorMessage := fmt.Sprintf("Failed to create training data for job %s", jobSubmissionDetails.Name)

		var fileId string
		mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())
		if !jobSubmissionDetails.UseUploadedData {
			fileId, err = trgSvc.collectTrainingData(mlModelConfig, mlStorage, mlStorage.GetLocalTrainingDataBaseDir(), job.TrainingDataCollecting)
			if err != nil {
				trgJob.Msg = err.Error()
				trgJob.StatusCode = job.Failed
				_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
				return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, trgJob.Msg))
			}
		} else {
			fileId, err = trgSvc.uploadTrainingDataFile(mlStorage)
			if err != nil {
				trgSvc.service.LoggingClient().Warn(err.Error())
				// Update status
				trgJob.Msg = "Error while uploading training data: " + err.Error()
				trgJob.StatusCode = job.Failed
				_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
				return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, trgJob.Msg))
			}
		}

		trgJob.StatusCode = job.TrainingDataCollected
		trgJob.TrainingDataFileId = fileId
		trgJob.Msg = "Training data set created, fileId: " + fileId
		_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
		//Deprecate old training jobs
		trgSvc.service.LoggingClient().Infof("Marking old jobs with training data collected status as deprecated")
		_ = trgSvc.dbClient.MarkOldTrainingDataDeprecated(trgJob.MLModelConfigName, trgJob.Name)

		trgSvc.service.LoggingClient().Info("Training data file generated : " + fileId)

		return fileId, nil
	}
*/
func (trgSvc *TrainingDataService) initCreateTrainingData(mlModelConfig *config.MLModelConfig, jobSubmissionDetails *job.JobSubmissionDetails) (*job.TrainingJobDetails, hedgeErrors.HedgeError) {
	// Create a training job to track the status of training data file generation
	var trgJob job.TrainingJobDetails
	trgJob.Name = jobSubmissionDetails.Name

	// Update the modelVersion in the trainingConfig file that will be saved
	modelVersion, err := trgSvc.dbClient.GetLatestModelVersion(mlModelConfig.MLAlgorithm, mlModelConfig.Name)
	if err != nil {
		//trgJob.LoggingClient().Warnf("Error while getting models before registering the model, error:%v", err)
		modelVersion = 1
	} else {
		modelVersion += 1
	}

	mlModelConfig.ModelConfigVersion = modelVersion

	trgJob.MLModelConfig = mlModelConfig
	trgJob.MLModelConfigName = jobSubmissionDetails.MLModelConfigName
	trgJob.MLModelConfig = mlModelConfig
	trgJob.MLAlgorithm = mlModelConfig.MLAlgorithm
	trgJob.StatusCode = job.New
	trgJob.StartTime = time.Now().Unix()

	existingJob, err := trgSvc.dbClient.GetMLTrainingJob(trgJob.Name)
	if err == nil && trgSvc.overrideJob(existingJob) {
		//ok to delete and move ahead
		_, err = trgSvc.dbClient.UpdateMLTrainingJob(trgJob) // Update so we can re-use same
	} else {
		_, err = trgSvc.dbClient.AddMLTrainingJob(trgJob)
	}

	if err != nil {
		trgJob.StatusCode = job.Failed
		trgJob.Msg = err.Error()
		_, _ = trgSvc.dbClient.UpdateMLTrainingJob(trgJob)
		// return the original error
		return nil, err
	}
	return &trgJob, nil
}

func (trgSvc *TrainingDataService) CreateTrainingDataLocal(mlModelConfig *config.MLModelConfig, jobSubmissionDetails *job.JobSubmissionDetails) (string, hedgeErrors.HedgeError) {
	trgJob, err := trgSvc.initCreateTrainingData(mlModelConfig, jobSubmissionDetails)
	if err != nil {
		return "", err
	}
	errorMessage := fmt.Sprintf("Failed to create training data for job %s", jobSubmissionDetails.Name)

	var trgDataSetFileName string
	if !jobSubmissionDetails.UseUploadedData {
		dataCollector, err := trgSvc.GetDataCollector(mlModelConfig)
		if err != nil {
			trgJob.Msg = err.Error()
			trgJob.StatusCode = job.Failed
			_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, trgJob.Msg))
		}

		dataCollectorErr := dataCollector.Execute()
		if dataCollectorErr != nil {
			trgSvc.service.LoggingClient().Warn(dataCollectorErr.Error())
			// Update status
			trgJob.Msg = "Error while collecting training data: " + dataCollectorErr.Error()
			trgJob.StatusCode = job.Failed
			_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, trgJob.Msg))
		}
		trgDataSetFileName = dataCollector.GetMlStorage().GetTrainingDataFileName()
	} else {
		mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())
		trainingServErr := trgSvc.placeUploadedTrainingData(mlStorage)
		if trainingServErr != nil {
			trgSvc.service.LoggingClient().Warn(trainingServErr.Error())
			// Update status
			trgJob.Msg = "Error while placing uploaded training data: " + trainingServErr.Error()
			trgJob.StatusCode = job.Failed
			_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
			return "", trainingServErr
		}
		trgDataSetFileName = mlStorage.GetTrainingDataFileName()
	}

	//trgJob.TrainingDataFileName=trgDataSetFileName

	trgJob.StatusCode = job.TrainingDataCollected
	trgJob.Msg = "Training data set created, FileName: " + trgDataSetFileName
	_, _ = trgSvc.dbClient.UpdateMLTrainingJob(*trgJob)
	//Deprecate old training jobs
	trgSvc.service.LoggingClient().Infof("Marking old jobs with training data collected status as deprecated")
	_ = trgSvc.dbClient.MarkOldTrainingDataDeprecated(trgJob.MLModelConfigName, trgJob.Name)

	trgSvc.service.LoggingClient().Info("Training data file generated : " + trgDataSetFileName)

	return trgDataSetFileName, nil
}

func (trgSvc *TrainingDataService) CreateTrainingSampleAsync(mlModelConfig *config.MLModelConfig, dataSampleSize int) {

	dataCollector, err := trgSvc.GetDataCollector(mlModelConfig)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Data Collector could not be created, check trainingDataConfig. Error: %s", err.Error())
		return
	}

	//`filePath := dataCollector.GetMlStorage().GetLocalTrainingDataBaseDir()
	filePath := strings.TrimSuffix(dataCollector.GetMlStorage().GetLocalTrainingDataBaseDir(), "/data")
	statusFilePath := fmt.Sprintf("%s/sample/status.json", filePath)

	if !trgSvc.initStatusFile(statusFilePath) {
		return
	}

	trgSvc.service.LoggingClient().Info("Creating training data sample")
	sampleData, dataCollectorErr := dataCollector.GenerateSample(dataSampleSize)
	if dataCollectorErr != nil {
		trgSvc.service.LoggingClient().Warn(dataCollectorErr.Error())
		_ = trgSvc.updateStatusResponse(trainingdata.StatusFailed, dataCollectorErr.Error(), statusFilePath)
		return
	}
	if len(sampleData) < 1 {
		trgSvc.service.LoggingClient().Info("Sample data is empty")
		_ = trgSvc.updateStatusResponse(trainingdata.StatusFailed, "Sample data is empty", statusFilePath)
		return
	}

	// Save sampleData to a CSV file
	sampleDataFileName := fmt.Sprintf("%s/sample/%s_sample.json", filePath, mlModelConfig.Name)

	if dataCollectorErr = saveToJSONFile(sampleDataFileName, sampleData); dataCollectorErr != nil {
		trgSvc.service.LoggingClient().Infof("Error when saving sampleData to .json file: %s", dataCollectorErr.Error())
		_ = trgSvc.updateStatusResponse(trainingdata.StatusFailed, dataCollectorErr.Error(), statusFilePath)
		return
	}

	_ = trgSvc.updateStatusResponse(trainingdata.StatusCompleted, "", statusFilePath)
}

// GetDataCollector returns the new instance of DataCollector interface implementation
func (trgSvc *TrainingDataService) GetDataCollector(mlModelConfig *config.MLModelConfig) (trainingdata.DataCollectorInterface, hedgeErrors.HedgeError) {

	if trgSvc.dataCollector == nil {
		dataCollector := trgSvc.NewDataCollectionProcessor(mlModelConfig, trgSvc.service, trgSvc.appConfig)
		if dataCollector == nil {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("DataCollector for %s could not be created, check mlModelConfig", mlModelConfig.Name))
		}
		return dataCollector, nil
	}
	return trgSvc.dataCollector, nil
}

func (trgSvc *TrainingDataService) NewDataCollectionProcessor(mlModelConfig *config.MLModelConfig, service interfaces.ApplicationService, appConfig *config.MLMgmtConfig) *trainingdata.DataCollector {

	dataCollector := new(trainingdata.DataCollector)
	mlAlgoDefinition, err := trgSvc.dbClient.GetAlgorithm(mlModelConfig.MLAlgorithm)
	if err != nil {
		trgSvc.service.LoggingClient().Error(err.Error())
		return nil
	}

	if mlAlgoDefinition == nil {
		trgSvc.service.LoggingClient().Info("No algorithm definition found")
		return nil
	}

	dataCollector.PreProcessor = helpers.NewPreProcessor(trgSvc.service.LoggingClient(), mlModelConfig, mlAlgoDefinition, true)
	if dataCollector.PreProcessor == nil {
		return nil
	}
	dataCollector.LoggingClient = service.LoggingClient()
	dataCollector.AppConfig = appConfig
	// This part of the query doesn't change, so build it once

	if mlModelConfig.MLDataSourceConfig.FeaturesByProfile == nil {
		trgSvc.service.LoggingClient().Warn("no features by profile configured, so not building the query")
	} else {
		dataCollector.BaseQuery = trainingdata.BuildQuery(mlModelConfig)
		trgSvc.service.LoggingClient().Infof("dataCollector.BaseQuery: %s", dataCollector.BaseQuery)
	}

	dataCollector.ChunkSize = int(BATCH_SIZE) // Read from config
	dataCollector.MlStorage = helpers.NewMLStorage(appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, service.LoggingClient())
	dataCollector.DataStoreProvider = trgSvc.dataStoreProvider
	//dataCollector.dataSourceRangeQueryUrl = appConfig.TrainingDataAPIUrl +"/query_range"
	return dataCollector
}

func (trgSvc *TrainingDataService) GetAppConfig() *config.MLMgmtConfig {
	return trgSvc.appConfig
}

func (trgSvc *TrainingDataService) GetDbClient() redis.MLDbInterface {
	return trgSvc.dbClient
}

func (trgSvc *TrainingDataService) GetDataStoreProvider() service.DataStoreProvider {
	return trgSvc.dataStoreProvider
}

func (trgSvc *TrainingDataService) GetService() interfaces.ApplicationService {
	return trgSvc.service
}

func (trgSvc *TrainingDataService) SaveUploadedTrainingData(mlModelConfig *config.MLModelConfig, uploadedFile multipart.File) hedgeErrors.HedgeError {
	mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())

	tmpTrainingDataFile := filepath.Join(mlStorage.GetValidationLocalDir(), "training_data.zip")

	err := helpers.SaveMultipartFile(uploadedFile, tmpTrainingDataFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to save uploaded training data in %s. Error: %v", tmpTrainingDataFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to save uploaded training data")
	}

	// Make sure the file is a valid .zip file
	zipReader, err := zip.OpenReader(tmpTrainingDataFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to read training data file %s. Error: %v", tmpTrainingDataFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to read training data file: not a valid .zip file")
	}
	zipReader.Close()

	// Step to clean Mac-specific attributes in the zip file
	cleanedTrainingDataFile := filepath.Join(mlStorage.GetValidationLocalDir(), "cleaned_training_data.zip")
	cmd := trgSvc.cmdRunner.GetCmd("sh", "/usr/local/bin/clean_zip.sh", tmpTrainingDataFile, cleanedTrainingDataFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = trgSvc.cmdRunner.Run(cmd)
	if err != nil {
		trgSvc.service.LoggingClient().Warnf("Failed to execute script. Error: %v. Stderr: %s", err, stderr.String())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to clean Mac-specific attributes from zip file")
	} else {
		trgSvc.service.LoggingClient().Infof("Succeeded to clean up .zip file %s. Stdout: %s", tmpTrainingDataFile, stdout.String())
	}

	// Use the cleaned zip file for further processing
	filesInZip, err := helpers.ReadZipFiles(cleanedTrainingDataFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to read zip files. Error: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to open zip file")
	}

	var configJson helpers.FileInZipInfo
	var csv helpers.FileInZipInfo

	for _, file := range filesInZip {
		ext := filepath.Ext(file.Path)
		name := filepath.Base(file.Path)

		if ext == ".csv" {
			csv = file
		}
		if name == CONFIG_FILE {
			configJson = file
		}
	}

	if configJson.Size == 0 {
		trgSvc.service.LoggingClient().Error(fmt.Sprintf("Missing or empty %s file", CONFIG_FILE))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Missing or empty %s file", CONFIG_FILE))
	}
	if csv.Size == 0 {
		trgSvc.service.LoggingClient().Error("Missing or empty csv file")
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Missing or empty csv file")
	}

	trainingDataFile := filepath.Join(mlStorage.GetValidationLocalDir(), "training_data.csv")
	trainingConfigFile := filepath.Join(mlStorage.GetValidationLocalDir(), CONFIG_FILE)

	err = helpers.ExtractZipFile(cleanedTrainingDataFile, csv.Path, trainingDataFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to extract csv file %s from uploaded zip file %s. Error: %v", csv.Path, cleanedTrainingDataFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to extract csv")
	}
	err = helpers.ExtractZipFile(cleanedTrainingDataFile, configJson.Path, trainingConfigFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to extract %s file %s from uploaded zip file %s. Error: %v", CONFIG_FILE, configJson.Path, cleanedTrainingDataFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Failed to extract %s", CONFIG_FILE))
	}
	return nil
}

func (trgSvc *TrainingDataService) ValidateUploadedData(mlModelConfig *config.MLModelConfig, mlAlgorithmDefinition *config.MLAlgorithmDefinition) (string, hedgeErrors.HedgeError) {
	mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())
	var compositeConfig config.MLConfig
	trainingConfigFile := filepath.Join(mlStorage.GetValidationLocalDir(), CONFIG_FILE)

	jsonData, err := helpers.ReadFileData(trainingConfigFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to read extracted %s %s. Error: %v", CONFIG_FILE, trainingConfigFile, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Failed to read extracted %s", CONFIG_FILE))
	}

	err = json.Unmarshal(jsonData, &compositeConfig)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to unmarshal %s %s. Error: %v", CONFIG_FILE, trainingConfigFile, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Failed to unmarshal %s", CONFIG_FILE))
	}

	// compare FeaturesByProfile
	existingFeaturesByProfile := mlModelConfig.MLDataSourceConfig.FeaturesByProfile
	if len(mlModelConfig.MLDataSourceConfig.ExternalFeatures) > 0 {
		existingFeaturesByProfile["external"] = mlModelConfig.MLDataSourceConfig.ExternalFeatures
	}
	uploadedFeaturesByProfile := compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile

	var added []string
	var missing []string
	var mismatched []string

	for key, uploadedFeatures := range uploadedFeaturesByProfile {
		existingFeatures, exists := existingFeaturesByProfile[key]
		if !exists {
			for _, uploadedFeature := range uploadedFeatures {
				added = append(added, uploadedFeature.Name)
			}
			continue
		}

		for _, uploadedFeature := range uploadedFeatures {
			var found bool
			for _, existingFeature := range existingFeatures {
				if uploadedFeature.Name == existingFeature.Name {
					found = true
					if uploadedFeature != existingFeature {
						mismatched = append(mismatched, uploadedFeature.Name)
					}
					break
				}
			}
			if !found {
				added = append(added, uploadedFeature.Name)
			}
		}
	}

	for key, existingFeatures := range existingFeaturesByProfile {
		uploadedFeatures, exists := uploadedFeaturesByProfile[key]
		if !exists {
			for _, existingFeature := range existingFeatures {
				missing = append(missing, existingFeature.Name)
			}
			continue
		}

		for _, existingFeature := range existingFeatures {
			var found bool
			for _, uploadedFeature := range uploadedFeatures {
				if uploadedFeature.Name == existingFeature.Name {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, existingFeature.Name)
			}
		}
	}
	var message string
	if len(added) != 0 || len(missing) != 0 || len(mismatched) != 0 {
		message = "The uploaded model configuration doesn't match the stored one. Please fix it and try again."

		if len(added) != 0 {
			message += " Added features: " + strings.Join(added, ",") + "."
		}
		if len(missing) != 0 {
			message += " Missing features: " + strings.Join(missing, ",") + "."
		}
		if len(mismatched) != 0 {
			message += " Mismatched features: " + strings.Join(mismatched, ",") + "."
		}
	}
	return message, nil
}

//TODO: restore after the release
/*
func (trgSvc *TrainingDataService) ValidateUploadedData(mlModelConfig *config.MLModelConfig, mlAlgorithmDefinition *config.MLAlgorithmDefinition) (hedgeErrors.HedgeError, *config.MLConfigValidation) {
	mlStorage := helpers.NewMLStorage(trgSvc.appConfig.BaseTrainingDataLocalDir, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service.LoggingClient())
	var compositeConfig config.MLConfig
	trainingConfigFile := filepath.Join(mlStorage.GetValidationLocalDir(), CONFIG_FILE)

	jsonData, err := helpers.ReadFileData(trainingConfigFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to read extracted %s %s. Error: %v", CONFIG_FILE, trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Failed to read extracted %s", CONFIG_FILE)), nil
	}

	err = json.Unmarshal(jsonData, &compositeConfig)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to unmarshal %s %s. Error: %v", CONFIG_FILE, trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Failed to unmarshal %s", CONFIG_FILE)), nil
	}

	mlConfigValidation := config.MLConfigValidation{
		ExistingMLConfig: config.MLConfig{
			MLModelConfig:    *mlModelConfig,
			MLAlgoDefinition: *mlAlgorithmDefinition,
		},
		NewMLConfig: compositeConfig,
	}

	// compare FeaturesByProfile
	existingFeatureByProfile := mlModelConfig.MLDataSourceConfig.FeaturesByProfile
	uploadedFeaturesByProfile := compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile

	if len(existingFeatureByProfile) != len(uploadedFeaturesByProfile) {
		trgSvc.service.LoggingClient().Warnf("Length of the existing featursByProfile %d doesn't match the length of the uploaded ones %d",
			len(existingFeatureByProfile), len(uploadedFeaturesByProfile))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
	}

	for key, uploadedFeatures := range uploadedFeaturesByProfile {
		existingFeatures, exists := existingFeatureByProfile[key]
		if !exists {
			trgSvc.service.LoggingClient().Warnf("Features %s don't exist in existing model config", key)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
		}
		if len(uploadedFeatures) != len(existingFeatures) {
			trgSvc.service.LoggingClient().Warnf("Length of the existing featurs %d doesn't match the length of the uploaded ones %d", len(existingFeatures), len(uploadedFeatures))
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
		}

		for _, uploadedFeature := range uploadedFeatures {
			var found bool
			for _, existingFeature := range existingFeatures {
				if uploadedFeature.Name == existingFeature.Name {
					found = true
					if uploadedFeature != existingFeature {
						trgSvc.service.LoggingClient().Warnf("Existing feature %v doesn't match the uploaded one %v", existingFeature, uploadedFeature)
						return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
					}
					break
				}
			}
			if !found {
				trgSvc.service.LoggingClient().Warnf("Feature %s is not found in the existing features", uploadedFeature.Name)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
			}
		}
	}

	// compare featureNameToColumnIndex
	existingFeatureNameToColumnIndex := mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex
	uploadedFeatureNameToColumnIndex := compositeConfig.MLModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex

	if len(existingFeatureNameToColumnIndex) != len(uploadedFeatureNameToColumnIndex) {
		trgSvc.service.LoggingClient().Warnf("Length of the existing FeatureNameToColumnIndex %d doesn't match the length of the uploaded one %d",
			len(existingFeatureNameToColumnIndex), len(uploadedFeatureNameToColumnIndex))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
	}

	for name, uploadedColumnIndex := range uploadedFeatureNameToColumnIndex {
		existingColumnIndex, exists := existingFeatureNameToColumnIndex[name]
		if !exists {
			trgSvc.service.LoggingClient().Warnf("Feature %s doesn't exist in existing model config", name)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
		}

		if existingColumnIndex != uploadedColumnIndex {
			trgSvc.service.LoggingClient().Warnf("Existing column index %v for feature %s doesn't match the uploaded one %v", name, existingColumnIndex, uploadedColumnIndex)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""), &mlConfigValidation
		}
	}

	return nil, &mlConfigValidation
}
*/
func (trgSvc *TrainingDataService) placeUploadedTrainingData(mlStorage helpers.MLStorageInterface) hedgeErrors.HedgeError {
	trainingDataTmpFile := filepath.Join(mlStorage.GetValidationLocalDir(), "training_data.csv")
	trainingConfigTmpFile := filepath.Join(mlStorage.GetValidationLocalDir(), CONFIG_FILE)
	trainingDataFile := mlStorage.GetTrainingDataFileName()
	trainingConfigFile := mlStorage.GetMLModelConfigFileName(true)

	errorMessage := "Failed to move uploaded training data"

	err := helpers.CopyFile(trainingDataTmpFile, trainingDataFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to copy training data tmp file %s to %s. Error: %v", trainingDataTmpFile, trainingDataFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	err = helpers.CopyFile(trainingConfigTmpFile, trainingConfigFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to copy %s tmp file %s to %s. Error: %v", CONFIG_FILE, trainingConfigTmpFile, trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return nil
}

func (trgSvc *TrainingDataService) generateConfig(mlModelConfig *config.MLModelConfig, trainingConfigFile string, mlStorage helpers.MLStorageInterface) hedgeErrors.HedgeError {
	// Write the training config file; kind of take a snapshot of the config at that time
	// Update trainingData config with algo information

	mlAlgoDefinition, hedgeErr := trgSvc.dbClient.GetAlgorithm(mlModelConfig.MLAlgorithm)
	if hedgeErr != nil {
		return hedgeErr
	}

	err := mlStorage.InitializeFile(trainingConfigFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to init training config file %s. Error: %v", trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to create training config file")
	}

	// Create a composite ML config containing both the ML model config and the ML algorithm definition
	compositeConfig := config.MLConfig{
		MLModelConfig:    *mlModelConfig,
		MLAlgoDefinition: *mlAlgoDefinition,
	}

	// Marshal the composite config to JSON
	bytes, err := json.MarshalIndent(compositeConfig, "", " ")
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to marshal compositeConfig for training config file %s. Error: %v", trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to create training config file")
	}
	err = os.WriteFile(trainingConfigFile, bytes, 0644)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("Failed to write file %s. Error: %v", trainingConfigFile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to create training config file")
	}
	return nil
}

/*
func (trgSvc *TrainingDataService) collectTrainingData(mlModelConfig *config.MLModelConfig, mlStorage helpers.MLStorageInterface, tmpDir string, collectingType string) (string, hedgeErrors.HedgeError) {
	if trgSvc.appConfig.TrainingProvider != "ADE" && trgSvc.appConfig.TrainingProvider != "AIF" {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. TrainingProvider %s is not supported", trgSvc.appConfig.TrainingProvider)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Invalid training provider")
	}

	errorMsg := "Collect training data failed"

	adeConnectionHandler := trgSvc.connectionHandler.(*service.ADEConnection)

	configJsonFile := filepath.Join(tmpDir, "config.json")

	hedgeErr := trgSvc.generateConfig(mlModelConfig, configJsonFile, mlStorage)
	if hedgeErr != nil {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Failed to generate config. Error: %v", hedgeErr.Error())
		return "", hedgeErr
	}

	template, err := ade.GetTemplate(trgSvc.service, adeConnectionHandler, helpers.DATA_COLLECTOR_NAME)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Failed to get template id for algo %s from AIF. Error: %v", helpers.DATA_COLLECTOR_NAME, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}
	var templateId string
	if template["id"] != nil {
		templateId = template["id"].(string)
	}
	if templateId == "" {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Template id for algo %s doesn't exist in AIF", helpers.DATA_COLLECTOR_NAME)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}

	instanceName := helpers.DATA_COLLECTOR_NAME + "_" + collectingType + "_" + mlModelConfig.MLAlgorithm + "_" + mlModelConfig.Name

	instanceId, err := ade.GetAIFAlgoInstanceId(adeConnectionHandler, templateId, mlModelConfig.MLAlgorithm, mlModelConfig.Name, trgSvc.service, "res/ade/hedge_algo_instance_dataCollector.json", instanceName)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Failed to get instanceId. Error: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}

	fileId, err := adeConnectionHandler.UploadFileToADE(mlStorage.GeneratorRemoteConfigJsonFileName(), configJsonFile)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Failed to upload config.json to ADE. Error: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}

	jobInput := make(map[string]map[string]interface{})
	jobInput["input"] = make(map[string]interface{})
	jobInput["input"]["config_json_fileId"] = fileId
	jobInput["input"]["collecting_type"] = collectingType

	executionId, err := ade.SubmitJob(trgSvc.service, instanceId, jobInput, "data collecting", adeConnectionHandler)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("collectTrainingData failed. Failed to submit job. Error: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}
	trgSvc.dataCollectorExecutionId = executionId
	trainingJob := &job.TrainingJobDetails{}
	err = MonitorTrainingJob(trgSvc.service.LoggingClient(), trgSvc.appConfig, trainingJob, trgSvc.getJobStatus)
	if err != nil {
		trgSvc.service.LoggingClient().Errorf("GenerateTrainingData failed. Job monitor failed. Error: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMsg)
	}

	return fileId, nil
}

func (trgSvc *TrainingDataService) downloadFile(fileId string, localFile string) error {
	if trgSvc.appConfig.TrainingProvider != "ADE" && trgSvc.appConfig.TrainingProvider != "AIF" {
		trgSvc.service.LoggingClient().Errorf("downloadFile failed. TrainingProvider %s is not supported", trgSvc.appConfig.TrainingProvider)
		return errors.New("Invalid training provider")
	}

	adeConnectionHandler := trgSvc.connectionHandler.(*service.ADEConnection)

	err := ade.DownloadFile(trgSvc.service.LoggingClient(), nil, adeConnectionHandler, fileId, localFile)
	return err
}



func (trgSvc *TrainingDataService) getJobStatus(job *job.TrainingJobDetails) error {
	adeConnectionHandler := trgSvc.connectionHandler.(*service.ADEConnection)
	err := ade.GetJobStatus(trgSvc.service, adeConnectionHandler, trgSvc.dataCollectorExecutionId, job)
	return err
}
*/

func saveToJSONFile(filename string, data []string) error {
	dataToSave, errStr := helpers.ConvCSVArrayToJSON(data)
	if errStr != "" {
		return errors.New(errStr)
	}
	err := helpers.SaveFile(filename, dataToSave)
	if err != nil {
		return err
	}
	return nil
}
