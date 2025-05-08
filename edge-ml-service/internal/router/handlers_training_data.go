package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	hedgeErrors "hedge/common/errors"

	"github.com/labstack/echo/v4"
	"hedge/edge-ml-service/pkg/dto/job"
)

/*
	func (r *Router) createTrainingData(c echo.Context) *echo.HTTPError {
		errorMessage := "Failed to create training data"
		lc := r.service.LoggingClient()

		lc.Info("Request to create training data")
		// We will use the job details to track the completion of data generation
		var jobSubmissionDetails job.JobSubmissionDetails

		err := json.NewDecoder(c.Request().Body).Decode(&jobSubmissionDetails)
		if err != nil {
			lc.Errorf("%s: %v", errorMessage, err)
			return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
		}

		if len(jobSubmissionDetails.Name) > 0 {
			mlModelConfig := jobSubmissionDetails.MLModelConfigName
			mlAlgorithm := jobSubmissionDetails.MLAlgorithm
			if len(mlModelConfig) > 0 {
				//  what is the file Name?,location of the file? how do user download it?
				trainingDataConfig, err := r.mlModelConfigService.GetMLModelConfig(
					mlAlgorithm,
					mlModelConfig,
				)
				if err != nil {
					lc.Errorf("%s: %v", errorMessage, err)
					if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
						return echo.NewHTTPError(
							http.StatusNotFound,
							fmt.Sprintf("ML model config '%s' not found", mlModelConfig),
						)
					} else {
						return echo.NewHTTPError(http.StatusInternalServerError, errorMessage)
					}
				}

				err = r.trainingDataService.CheckIfJobExists(jobSubmissionDetails.Name)
				if err != nil {
					lc.Errorf("%s: %v", errorMessage, err)
					return echo.NewHTTPError(http.StatusAlreadyReported, err.Error())
				}
				// Asynchronous call, as training data generation may take time
				go func() {
					_, err = r.trainingDataService.CreateTrainingData(
						trainingDataConfig,
						&jobSubmissionDetails,
					)
					if err != nil {
						lc.Error(err.Error())
					}
				}()

				_ = c.NoContent(http.StatusAccepted)
			} else {
				lc.Errorf("%s: %s", errorMessage, "Training data configuration name should not be empty")
				return echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("%s: %s", errorMessage, "Training data configuration name should not be empty"))
			}
		} else {
			lc.Errorf("%s: %s", errorMessage, "Training dataset name should not be empty")
			return echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("%s: %s", errorMessage, "Training dataset name should not be empty"))
		}

		return nil
	}
*/
func (r *Router) createTrainingDataExport(c echo.Context) *echo.HTTPError {
	lc := r.service.LoggingClient()
	lc.Info("Request to create training data export")

	errorMessage := "Failed to create training data export"

	var jobSubmissionDetails job.JobSubmissionDetails

	err := json.NewDecoder(c.Request().Body).Decode(&jobSubmissionDetails)
	if err != nil {
		lc.Errorf("Failed to decode request body into jobSubmissionDetails struct  %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf("%s: %s", errorMessage, "invalid request entity"),
		)
	}
	hedgeErr := r.validateMlAlgorithmEnabled(
		jobSubmissionDetails.MLAlgorithm,
		fmt.Sprintf(
			"Create training data for model config %s",
			jobSubmissionDetails.MLModelConfigName,
		),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	generateConfigOnlyStr := c.QueryParam("generateConfigOnly")
	generateConfigOnly := false
	if generateConfigOnlyStr != "" {
		var err error
		generateConfigOnly, err = strconv.ParseBool(generateConfigOnlyStr)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Error parsing generateConfigOnly flag: %s", err.Error())
			return echo.NewHTTPError(
				http.StatusBadRequest,
				"generateConfigOnly param should be boolean",
			)
		}
	}
	mlModelConfigName := jobSubmissionDetails.MLModelConfigName
	mlAlgorithm := jobSubmissionDetails.MLAlgorithm
	//  what is the file Name?,location of the file? how do user download it?
	mlModelConfig, hErr := r.mlModelConfigService.GetMLModelConfig(mlAlgorithm, mlModelConfigName)
	if hErr != nil {
		r.service.LoggingClient().Error(hErr.Error())
		if hErr.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to get ML model config, config "+mlModelConfigName+" not found.",
			)
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
		}
	}
	// Asynchronous call for training data export
	go r.trainingDataService.CreateTrainingDataExport(mlModelConfig, generateConfigOnly)

	// Immediately return
	_ = c.NoContent(http.StatusAccepted)
	return nil
}

func (r *Router) getTrainingDataExportStatus(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Debug("Request to get status of training data export")

	statusFilePath, err := resolveTrainingFilePath(
		c,
		r.trainingDataService.GetAppConfig(),
		"status.json",
		"data_export",
	)
	if err != nil {
		r.service.LoggingClient().Errorf("Error resolving status file path: %s", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "Error reading training file")
	}

	// Read the status.json file
	statusContent, err := os.ReadFile(statusFilePath)
	if err != nil {
		r.service.LoggingClient().
			Warnf("Failed to read status.json file, will return NOT_FOUND, error: %s", err.Error())
		statusNotFound := map[string]string{"status": "NOT_FOUND"}
		statusContent, _ = json.Marshal(statusNotFound)
	}
	var statusResponse map[string]interface{}
	err = json.Unmarshal(statusContent, &statusResponse)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid status file format")
	}

	_ = c.JSON(http.StatusOK, statusResponse)
	return nil
}

func (r *Router) getTrainingDataExport(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to get training data export")

	exportFileName := "training_input.zip"

	filePath, err := resolveTrainingFilePath(
		c,
		r.trainingDataService.GetAppConfig(),
		exportFileName,
		"data_export",
	)
	if err != nil {
		r.service.LoggingClient().Errorf("Error resolving status file path: %s", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "Error reading training file")
	}

	file, err := os.Open(filePath)
	if err != nil {
		r.service.LoggingClient().Errorf("Error opening status file path: %s", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to open training file")
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		r.service.LoggingClient().Errorf("Error read status file: %s", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read training file")
	}

	fileSize := stat.Size()

	c.Response().
		Header().
		Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", exportFileName))
	c.Response().Header().Set("Content-Type", "multipart/form-data")
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
	// Serve the file
	_ = c.Stream(http.StatusOK, "multipart/form-data", file)
	return nil
}

func (r *Router) createTrainingDataSample(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to create training data sample")

	mlModelConfigName := c.QueryParam("mlModelConfigName")
	if mlModelConfigName == "" {
		// only for temp backward compatibility till UI is fixed
		mlModelConfigName = c.QueryParam("mlModelConfigName")
	}
	// mlModelConfigName
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlAlgorithm,
		fmt.Sprintf("Create training data sample for model config %s", mlModelConfigName),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	sampleSize := c.QueryParam("sampleSize")
	if sampleSize == "" {
		sampleSize = "100"
	}
	sampleSizeInt, err := strconv.Atoi(sampleSize)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to convert sampleSize to int: %s", err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid format for sampleSize")
	}

	if len(mlModelConfigName) > 0 {
		mlModelConfig, err := r.mlModelConfigService.GetMLModelConfig(
			mlAlgorithm,
			mlModelConfigName,
		)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to get ML model config for algo %s, model config %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				return echo.NewHTTPError(
					http.StatusNotFound,
					"Unable to get ML model config, config "+mlAlgorithm+" not found.",
				)
			} else {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
			}
		}

		// Asynchronous call for training data sample
		go r.trainingDataService.CreateTrainingSampleAsync(mlModelConfig, sampleSizeInt)

		// Immediately return
		_ = c.NoContent(http.StatusAccepted)
	} else {
		return echo.NewHTTPError(http.StatusConflict, "mlModelConfigName parameter should not be empty")
	}
	return nil
}

func (r *Router) getTrainingDataSampleStatus(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Debug("Request to get status of training data sample")

	statusFilePath, err := resolveTrainingFilePath(
		c,
		r.trainingDataService.GetAppConfig(),
		"status.json",
		"sample",
	)
	if err != nil {
		r.service.LoggingClient().Errorf("Error resolving status file path: %s", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "Error reading training file")
	}

	// Read the status.json file
	statusContent, err := os.ReadFile(statusFilePath)
	if err != nil {
		r.service.LoggingClient().
			Warnf("Failed to read status.json file, will return NOT_FOUND, error: %s", err.Error())
		statusContent, _ = json.Marshal(map[string]string{"status": "NOT_FOUND"})
	}
	var statusResponse map[string]interface{}
	err = json.Unmarshal(statusContent, &statusResponse)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Invalid json file")
	}

	_ = c.JSON(http.StatusOK, statusResponse)
	return nil
}

func (r *Router) getTrainingDataSample(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to get training data sample")
	// Extract and sanitize mlModelConfigName from the query string
	mlModelConfig := filepath.Clean(c.QueryParam("mlModelConfigName"))

	// Construct the sanitized sample data file name
	sampleDataFileName := fmt.Sprintf("%s_sample.json", mlModelConfig)

	filePath, err := resolveTrainingFilePath(
		c,
		r.trainingDataService.GetAppConfig(),
		sampleDataFileName,
		"sample",
	)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Error resolving %s file path: %s", sampleDataFileName, err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, "Error reading training file")
	}

	// Read the sample data file
	sampleDataJson, err := os.ReadFile(filePath)
	if err != nil {
		r.service.LoggingClient().
			Error(fmt.Sprintf("Failed to read sample data file: %s", err.Error()))
		return echo.NewHTTPError(http.StatusInternalServerError, "Error reading training file")
	}

	// Validate JSON structure
	var temp interface{}
	if err = json.Unmarshal(sampleDataJson, &temp); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Invalid JSON file")
	}

	_ = c.Blob(http.StatusOK, "application/json", sampleDataJson)
	return nil
}

func (r *Router) uploadAndValidateTrainingData(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Request to validate training data")

	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")

	if len(mlModelConfigName) == 0 {
		r.service.LoggingClient().Errorf("mlModelConfigName is required")
		return echo.NewHTTPError(http.StatusBadRequest, "mlModelConfigName should not be Empty")
	}
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlAlgorithm,
		fmt.Sprintf("Upload training data for model config %s", mlModelConfigName),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	mlModelConfig, err := r.mlModelConfigService.GetMLModelConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to get model config for algo %s, model config %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to get ML model config, config "+mlModelConfigName+" not found.",
			)
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get model config with name "+mlModelConfigName+" for algorithm"+mlAlgorithm)
		}
	}

	mlAlgorithmDefinition, err := r.mlModelConfigService.GetAlgorithm(mlAlgorithm)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to get algorithm %s. Error: %v", mlAlgorithm, err)
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to get algorithm, config "+mlAlgorithm+" not found.",
			)
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get algorithm "+mlAlgorithm)
		}
	}

	fileHeader, ctxError := c.FormFile("file")
	if ctxError != nil {
		r.service.LoggingClient().Errorf("Failed to get file header. Error: %v", ctxError)
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to read file")
	}
	tmpFile, ctxError := fileHeader.Open()
	if ctxError != nil {
		r.service.LoggingClient().Errorf("Failed to open file for writing: %s", fileHeader.Filename)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to open file")
	}
	defer tmpFile.Close()

	hedgeError := r.trainingDataService.SaveUploadedTrainingData(mlModelConfig, tmpFile)
	if hedgeError != nil {
		return hedgeError.ConvertToHTTPError()
	}

	message, hedgeError := r.trainingDataService.ValidateUploadedData(
		mlModelConfig,
		mlAlgorithmDefinition,
	)
	if hedgeError != nil {
		return hedgeError.ConvertToHTTPError()
	}
	if message != "" {
		return echo.NewHTTPError(http.StatusExpectationFailed, message)
	}

	_ = c.NoContent(http.StatusAccepted)

	return nil
}
