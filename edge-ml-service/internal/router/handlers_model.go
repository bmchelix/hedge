package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"

	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/edge-ml-service/pkg/helpers"
	"github.com/labstack/echo/v4"
)

// 35MB - safe for the max_payload defined in nats_server.conf (64MB): NATS messages include protocol overhead (eg http response size in nats-proxy)
// 2 chunks is max for 64MB (more chunks == more polling calls)
const chunkSize = 35 * 1024 * 1024

var modelChunks sync.Map

func (r *Router) uploadAndRegisterModel(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to upload trained model")

	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	jName := c.QueryParam("jobName")
	description := c.QueryParam("description")
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlAlgorithm,
		fmt.Sprintf("Upload model for model config %s", mlModelConfigName),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	jobSubmissionDetails := job.JobSubmissionDetails{
		Name:              jName,
		Description:       description,
		MLAlgorithm:       mlAlgorithm,
		MLModelConfigName: mlModelConfigName,
	}
	var jobDetails *job.TrainingJobDetails
	if len(jobSubmissionDetails.Name) != 0 {
		var err hedgeErrors.HedgeError
		jobDetails, err = r.trainingJobService.BuildJobConfig(jobSubmissionDetails)
		if err != nil {
			r.service.LoggingClient().Errorf("Failed to build job %s. Error: %v", jName, err)
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				return echo.NewHTTPError(
					http.StatusNotFound,
					fmt.Sprintf("Job %s not found", jName),
				)
			} else {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to build training job details for jon "+jName)
			}
		}
	} else {
		return echo.NewHTTPError(http.StatusConflict, "Job name should not be empty")
	}

	fileName := helpers.MODEL_ZIP_FILENAME

	fileHeader, err := c.FormFile(fileName)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to read file: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read file")
	}
	tmpFilePath, err := resolveTrainingFilePath(
		c,
		r.trainingDataService.GetAppConfig(),
		"hedge_export.tmp.zip",
		"",
	)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to read file path: %v", err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Failed to resolve training file path",
		)
	}
	// Open the file
	tmpFile, err := fileHeader.Open()
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to open file header: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to open training file")
	}
	defer tmpFile.Close()
	err = r.trainingJobService.RegisterUploadedModel(tmpFilePath, tmpFile, jobDetails)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to register model: %v", err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Failed to register model "+mlAlgorithm,
		)
	}
	_ = c.NoContent(http.StatusCreated)
	return nil
}

func (r *Router) downloadModelFile(c echo.Context) *echo.HTTPError {
	lc := r.service.LoggingClient()
	lc.Info("Request to get trained Model file")
	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	chunkIndex := c.QueryParam("chunkIndex")

	if len(mlModelConfigName) > 0 {
		// Synchronous call, will time out at UI, what is the file Name?,location of the file? how do user download it?
		_, err := r.mlModelConfigService.GetMLModelConfig(mlAlgorithm, mlModelConfigName)
		if err != nil {
			lc.Errorf("Failed to get ML model config for algo %s, model config %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Unable to get ML model config, config "+mlAlgorithm+" not found.")
			} else {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
			}
		}

		modelID := mlModelConfigName + "_" + mlAlgorithm
		chunksInterface, found := modelChunks.Load(modelID)
		if !found {
			localTrainedModelFile := helpers.NewMLStorage(r.appConfig.BaseTrainingDataLocalDir, mlAlgorithm, mlModelConfigName, r.service.LoggingClient()).
				GetModelZipFile()
			lc.Infof("Preparing chunks...")
			err1 := prepareChunks(modelID, localTrainedModelFile, lc)
			if err1 != nil {
				lc.Errorf("Failed to prepare model chunks for model config with name %s, algorithm %s, err %s", mlModelConfigName, mlAlgorithm, err1.Error())
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to prepare model chunks for model config with name "+mlModelConfigName+", algorithm "+mlAlgorithm)
			}
			chunksInterface, _ = modelChunks.Load(modelID)
		}
		chunks := chunksInterface.([]ml_model.ChunkMessage)
		index, _ := strconv.Atoi(chunkIndex)
		if index >= len(chunks) {
			lc.Errorf("Failed to get chunk for model config with name %s, algorithm %s", mlModelConfigName, mlAlgorithm)
			return echo.NewHTTPError(http.StatusNotFound, "Failed to get chunk for model config with name "+mlModelConfigName+", algorithm "+mlAlgorithm)
		}

		var chunkData []byte
		chunkData, err1 := json.Marshal(chunks[index])
		if err1 != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed while marshalling chunk")
		}
		// If this is the last chunk, remove the entry from the map
		if index == chunks[index].TotalChunks-1 {
			modelChunks.Delete(modelID)
		}

		c.Response().WriteHeader(http.StatusOK)
		c.Response().Header().Set(ContentType, "application/octet-stream")
		_, _ = c.Response().Write(chunkData)
	} else {
		return echo.NewHTTPError(http.StatusConflict, "mlModelConfigName parameter should not be empty")
	}
	return nil
}

func prepareChunks(modelID, localTrainedModelFile string, lc logger.LoggingClient) error {
	file, err := os.Open(localTrainedModelFile)
	if err != nil {
		lc.Errorf("Failed to read model file %s", localTrainedModelFile)
		return fmt.Errorf("failed to open model file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		lc.Errorf("Failed to get file info %v", err)
		return fmt.Errorf("failed to get file info: %v", err)
	}

	fileSize := fileInfo.Size()
	lc.Infof("Model file size: %.2f MB", float64(fileSize)/1024.0/1024.0)

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	var chunks []ml_model.ChunkMessage

	for i := 0; i < totalChunks; i++ {
		buffer := make([]byte, chunkSize)
		bytesRead, err := io.ReadFull(file, buffer)
		if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("failed to read chunk: %v", err)
		}
		buffer = buffer[:bytesRead]

		lc.Infof("Total chunks: %d", totalChunks)
		lc.Infof("Chunk index: %d, size %.2f MB", i, float64(bytesRead/1024.0/1024.0))

		chunk := ml_model.ChunkMessage{
			ModelID:     modelID,
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			ChunkData:   buffer,
		}

		chunks = append(chunks, chunk)
	}

	modelChunks.Store(modelID, chunks)
	return nil
}

func (r *Router) deployOrUndeployModel(c echo.Context) *echo.HTTPError {
	lc := r.service.LoggingClient()
	lc.Info("Request to deploy trained Model to target nodes")

	var modelDeploymentCmd ml_model.ModelDeployCommand
	err := json.NewDecoder(c.Request().Body).Decode(&modelDeploymentCmd)
	if err != nil {
		lc.Errorf("Failed to decode request body into ModelDeployCommand struct  %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to validate request body")
	}

	if len(modelDeploymentCmd.TargetNodes) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Target deployment nodes cannot be empty")
	}
	// Synchronous call, will time out at UI, what is the file Name?,location of the file? how do user download it?
	_, hErr := r.mlModelConfigService.GetMLModelConfig(
		modelDeploymentCmd.MLAlgorithm,
		modelDeploymentCmd.MLModelConfigName,
	)
	if hErr != nil {
		lc.Errorf(
			"Failed to get ML model config for algo %s, model config %s. Error: %v",
			modelDeploymentCmd.MLAlgorithm,
			modelDeploymentCmd.MLModelConfigName,
			hErr,
		)
		if hErr.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to get ML model config, config "+modelDeploymentCmd.MLAlgorithm+" not found.",
			)
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model config with name "+modelDeploymentCmd.MLModelConfigName+" for algorithm"+modelDeploymentCmd.MLAlgorithm)

		}
	}

	algorithm, err := r.mlModelConfigService.GetAlgorithm(modelDeploymentCmd.MLAlgorithm)
	if err != nil || algorithm == nil {
		lc.Errorf(
			"Failed to get algorithm definition %s, error: %s",
			modelDeploymentCmd.MLAlgorithm,
			err.Error(),
		)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to get algorithm definition %s", modelDeploymentCmd.MLAlgorithm),
		)
	}

	// Verify that prediction image exists in registry, validate digest value
	if algorithm.PredictionImageDigest == "" {
		errMsg := fmt.Sprintf("Failed to deploy model: prediction image digest is not set for algo %s", algorithm.Name)
		lc.Errorf("%s, err %v", errMsg, err)
		return echo.NewHTTPError(http.StatusInternalServerError, errMsg)
	} else {
		actualPredictionImageDigest, err := r.mlModelConfigService.GetRegistryConfig().GetImageDigest(algorithm.PredictionImagePath)
		errMsgBase := fmt.Sprintf("Failed to deploy model: prediction image validation failed for '%s', image digest '%s', algorithm '%s'", algorithm.PredictionImagePath, algorithm.PredictionImageDigest, algorithm.Name)
		if err != nil {
			errMsg := fmt.Sprintf("%s, err: %s", errMsgBase, err.Error())
			lc.Errorf(errMsg)
			return echo.NewHTTPError(http.StatusInternalServerError, errMsg)
		}
		lc.Infof("actualPredictionImageDigest - %s", actualPredictionImageDigest)
		if actualPredictionImageDigest != algorithm.PredictionImageDigest {
			errMsg := fmt.Sprintf("%s, err: prediction image digest provided in algo definition doesn't exist in the image registry", errMsgBase)
			lc.Errorf(errMsg)
			return echo.NewHTTPError(http.StatusInternalServerError, errMsg)
		}
		lc.Infof("Successfully validated prediction image digest for algo %s", algorithm.Name)
	}

	err = r.trainingJobService.DeployUndeployMLModel(&modelDeploymentCmd, algorithm)
	if err != nil {
		lc.Errorf("Failed to deploy model: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to deploy model")
	}

	_ = c.NoContent(http.StatusAccepted)
	return nil
}

func (r *Router) getModelDeploymentStatus(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("request to get Model deployments")

	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	groupByNodesStr := c.QueryParam("groupByNodes")
	groupByNodes := true
	if groupByNodesStr == "false" {
		groupByNodes = false
	}
	deploymentStatus := c.QueryParam("deploymentStatus")

	var modelDeployments []ml_model.ModelDeploymentStatus
	var err error
	if groupByNodes {
		modelDeployments, err = r.trainingJobService.GetModelDeploymentsGroupedByNode(mlAlgorithm, mlModelConfigName, "all", deploymentStatus)
		if err != nil {
			return echo.NewHTTPError(
				http.StatusInternalServerError,
				"Failed to get model deployment config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm,
			)
		}
		if len(modelDeployments) == 0 {
			_ = c.JSON(http.StatusNotFound, modelDeployments)
			return nil
		}
	} else {
		modelDeployments, err = r.trainingJobService.GetModelDeploymentsByConfig(mlAlgorithm, mlModelConfigName, groupByNodes)
		if err != nil {
			r.service.LoggingClient().Errorf("Failed to get model deployment config: %s", err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model deployment config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
		}
	}

	_ = c.JSON(http.StatusOK, modelDeployments)

	return nil
}

func (r *Router) getModels(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to get all trained Models")

	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	latestVersionsOnlyStr := c.QueryParam("latestVersionsOnly")
	latestVersionOnly := true
	if latestVersionsOnlyStr == "false" {
		latestVersionOnly = false
	}

	models, err := r.trainingJobService.GetModels(mlAlgorithm, mlModelConfigName, latestVersionOnly)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to get ML model for algo %s, model config %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to get model, config "+mlAlgorithm+" not found.",
			)
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
		}
	}
	_ = c.JSON(http.StatusOK, models)
	return nil
}
