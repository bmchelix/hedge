package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"hedge/common/utils"
	"golang.org/x/sync/errgroup"

	hedgeErrors "hedge/common/errors"
	config2 "hedge/edge-ml-service/internal/config"

	"hedge/edge-ml-service/pkg/dto/config"
	"github.com/labstack/echo/v4"
)

type PredictionPortResponse struct {
	PortNo int64 `json:"portNo"`
}

func (r *Router) addAlgorithm(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Registering a new algorithm")
	var algorithm config.MLAlgorithmDefinition
	err := json.NewDecoder(c.Request().Body).Decode(&algorithm)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to decode request body into MLAlgorithmDefinition struct  %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to validate request body "+err.Error())
	}

	generateDigestStr := c.QueryParam("generateDigest")
	generateDigest := false
	if generateDigestStr != "" {
		generateDigest, err = strconv.ParseBool(generateDigestStr)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Invalid parameter generateDigest. Error while parsing: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid parameter generateDigest "+err.Error())
		}
	}

	userId, hedgeErr := utils.GetUserIdFromHeader(c.Request(), r.service)
	if hedgeErr != nil {
		r.service.LoggingClient().Errorf(hedgeErr.Message())
		//return hedgeErr.ConvertToHTTPError()
	}

	err = r.validate.Struct(algorithm)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to validate the structure of algorithm: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to validate the structure of algorithm:"+err.Error(),
		)
	}

	hedgeErr = r.mlModelConfigService.CreateAlgorithm(algorithm, generateDigest, userId)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}

	r.service.LoggingClient().Infof("Created algorithm definition successfully: %s", algorithm.Name)
	_ = c.NoContent(http.StatusCreated)
	return nil
}

func (r *Router) updateAlgorithm(c echo.Context) *echo.HTTPError {
	var algorithm config.MLAlgorithmDefinition
	err := json.NewDecoder(c.Request().Body).Decode(&algorithm)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to decode request body into MLAlgorithmDefinition struct  %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to validate request body "+err.Error())
	}

	generateDigestStr := c.QueryParam("generateDigest")
	generateDigest := false
	if generateDigestStr != "" {
		generateDigest, err = strconv.ParseBool(generateDigestStr)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Invalid parameter generateDigest. Error while parsing: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid parameter generateDigest "+err.Error())
		}
	}

	userId, hedgeErr := utils.GetUserIdFromHeader(c.Request(), r.service)
	if hedgeErr != nil {
		r.service.LoggingClient().Errorf(hedgeErr.Message())
		//return hedgeErr.ConvertToHTTPError()
	}

	err = r.validate.Struct(algorithm)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to validate the structure of algorithm: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to validate the structure of algorithm:"+err.Error(),
		)
	}

	hedgeErr = r.mlModelConfigService.UpdateAlgorithm(algorithm, generateDigest, userId)
	if hedgeErr != nil {
		r.service.LoggingClient().Errorf("Failed to update algorithm: %v", err)
		return hedgeErr.ConvertToHTTPError()
	}

	r.service.LoggingClient().Infof("Updated algorithm successfully: %s", algorithm.Name)
	r.service.LoggingClient().Infof("Triggering training data re-generation for all the ml model configs associated with the algo: %s", algorithm.Name)

	mlModelConfigsByAlgoName, err := r.getModelConfigs(algorithm.Name, "all", false)
	// Set export data deprecated for all model configs associated by the algorithm to keep config.json up-to-date for trainings
	for _, mlModelConfig := range mlModelConfigsByAlgoName.([]config.MLModelConfig) {
		r.service.LoggingClient().Infof("Setting data export deprecated for mlModelConfig: %s", mlModelConfig.Name)
		hedgeErr = r.trainingDataService.SetTrainingDataExportDeprecated(&mlModelConfig)
		if hedgeErr != nil && hedgeErr.ErrorType() != hedgeErrors.ErrorTypeNotFound {
			r.service.LoggingClient().Errorf("Failed to set data export deprecated for mlAlgorithm: %s, mlModelConfig: %s. Error: %s", mlModelConfig.MLAlgorithm, mlModelConfig.Name, hedgeErr.Message())
		}
	}

	_ = c.NoContent(http.StatusCreated)
	return nil
}

func (r *Router) getAlgorithmByName(c echo.Context) *echo.HTTPError {

	name := c.Param("algorithmName")
	if name == "" {
		r.service.LoggingClient().Errorf("Required query parameter 'algorithmName' is missing")
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Required query parameter 'algorithmName' is missing",
		)
	}

	summaryOnlyStr := c.QueryParam("summaryOnly")
	summaryOnly := false
	if summaryOnlyStr != "" {
		var err error
		summaryOnly, err = strconv.ParseBool(summaryOnlyStr)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Invalid parameter summaryOnly. Error while parsing: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid parameter summaryOnly")
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	algo, err := r.getAlgo(ctx, name, summaryOnly)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().
				Errorf("Could not get algorithm by name: %s. Error: %v", name, err)
			return err.ConvertToHTTPError()
		}
		r.service.LoggingClient().
			Errorf("Could not get algorithm by name: %s. Error: %v", name, err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Could not get algorithm by name: "+name,
		)
	}

	if err := c.JSON(http.StatusOK, algo); err != nil {
		r.service.LoggingClient().Errorf("Error creating json response with algorithm: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating json response.")
	}

	return nil
}

func (r *Router) deleteAlgorithm(c echo.Context) *echo.HTTPError {

	name := c.Param("algorithmName")
	if name == "" {
		r.service.LoggingClient().Errorf("Required query parameter 'algorithmName' is missing")
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Required query parameter 'algorithmName' is missing",
		)
	}

	hedgeErr := r.mlModelConfigService.DeleteAlgorithm(name)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	r.service.LoggingClient().Infof("Successfully deleted algorithm: %s", name)
	_ = c.NoContent(http.StatusOK)
	return nil
}

func (r *Router) changeAlgorithmStatus(c echo.Context) *echo.HTTPError {
	name := c.Param("algorithmName")
	if name == "" {
		r.service.LoggingClient().Errorf("Required query parameter 'algorithmName' is missing")
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Required query parameter 'algorithmName' is missing",
		)
	}
	isEnabledStr := c.QueryParam("isEnabled")
	if isEnabledStr == "" {
		r.service.LoggingClient().Errorf("Required query parameter 'isEnabled' is missing")
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Required query parameter 'isEnabled' is missing",
		)
	}
	isEnabled, err := strconv.ParseBool(isEnabledStr)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Invalid parameter 'isEnable'. Error while parsing: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid parameter 'isEnable")
	}
	hedgeErr := r.mlModelConfigService.ChangeAlgorithmStatus(name, isEnabled)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	enabledDisabledStr := "enabled"
	if !isEnabled {
		enabledDisabledStr = "disabled"
	}
	r.service.LoggingClient().
		Infof("Successfully changed status to %s for algorithm: %s", enabledDisabledStr, name)
	_ = c.NoContent(http.StatusOK)

	return nil
}

// Gets the next available port for allocation to algorith prediction container
func (r *Router) getAlgothmPredictionPort(c echo.Context) *echo.HTTPError {
	algoName := c.Param("algorithmName")
	if algoName == "" {
		r.service.LoggingClient().Errorf("Required query parameter 'algorithmName' is missing")
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Required query parameter 'algorithmName' is missing",
		)
	}
	portNo := r.mlModelConfigService.GetAllocatedPortNo()
	portNoMap := PredictionPortResponse{PortNo: portNo}
	r.service.LoggingClient().Infof("allocated port no %d for algorithm: %s", portNo, algoName)
	_ = c.JSON(http.StatusOK, portNoMap)
	return nil
}

func (r *Router) createMLModelConfiguration(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("POST request for ML model configuration")
	mlModelConfig, err := r.unmarshalToMLModelConfig(c.Request())
	if err != nil {
		r.service.LoggingClient().Errorf("Error unmarshalling request body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Error parsing request")
	}
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlModelConfig.MLAlgorithm,
		fmt.Sprintf("Create model config %s", mlModelConfig.Name),
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
		r.service.LoggingClient().Errorf("Error converting sampleSize to int: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Error converting sampleSize to int")
	}

	generatePreview := c.QueryParam("generatePreview")
	r.service.LoggingClient().Infof("Value of generatePreview queryParameter: %v", generatePreview)

	// Perform validation based on the MLModelConfig struct tags
	err = r.validate.Struct(mlModelConfig)
	if err != nil {
		r.service.LoggingClient().Errorf("Error validating ML model configuration: %v", err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Error validating ML model configuration",
		)
	}

	existingTrainingConfig, err := r.mlModelConfigService.GetMLModelConfig(
		mlModelConfig.MLAlgorithm,
		mlModelConfig.Name,
	)
	if err == nil && existingTrainingConfig != nil {
		r.service.LoggingClient().
			Errorf("Training config already exist: %v", existingTrainingConfig)
		return echo.NewHTTPError(
			http.StatusConflict,
			"Training config with name "+existingTrainingConfig.Name+" already exists, please use different name",
		)
	}

	err = r.mlModelConfigService.ValidateMLModelConfigFilters(
		mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters,
	)
	if err != nil {
		r.service.LoggingClient().Errorf("Error validating ML model configuration: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid filters for model configuration")
	}

	algorithm, err := r.mlModelConfigService.GetAlgorithm(mlModelConfig.MLAlgorithm)
	if err != nil || algorithm == nil {
		return echo.NewHTTPError(
			http.StatusNotFound,
			"Failed to save ML model config "+mlModelConfig.Name+": algorithm "+mlModelConfig.MLAlgorithm+" not found.",
		)
	}
	mlModelConfig.MLAlgorithmType = algorithm.Type
	// Build the feature index in here
	r.mlModelConfigService.BuildFeatureNameToIndexMap(
		&mlModelConfig,
		algorithm.TimeSeriesAttributeRequired,
		algorithm.GroupByAttributesRequired,
	)
	// Disabling the validation in case of create if there are no features defined - this applies in case of UI
	if len(mlModelConfig.MLDataSourceConfig.FeaturesByProfile) != 0 {
		err = r.mlModelConfigService.ValidateFeaturesToProfileByAlgoType(&mlModelConfig, true)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to save ML model config for algo %s, model config %s: validation failed. Error: %s", mlModelConfig.MLAlgorithm, mlModelConfig.Name, err.Error())
			return echo.NewHTTPError(
				http.StatusBadRequest,
				"Failed to save model config "+mlModelConfig.Name+" for algorithm "+mlModelConfig.MLAlgorithm+": error: "+err.Error(),
			)
		}
	}

	err = r.mlModelConfigService.SaveMLModelConfig(&mlModelConfig, algorithm)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to save ML model config for algo %s, model config %s. Error: %v", mlModelConfig.MLAlgorithm, mlModelConfig.Name, err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Failed to save ML model config "+mlModelConfig.Name+" for algorithm "+mlModelConfig.MLAlgorithm,
		)
	} else {
		// Asynchronous call for creating training data sample
		if generatePreview == "true" || generatePreview == "yes" {
			go r.trainingDataService.CreateTrainingSampleAsync(&mlModelConfig, sampleSizeInt)
		}
		_ = c.NoContent(http.StatusCreated)
	}

	return nil
}

func (r *Router) updateMLModelConfiguration(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("PUT request for General Training configuration")
	mlModelConfig, err := r.unmarshalToMLModelConfig(c.Request())
	if err != nil {
		r.service.LoggingClient().Errorf("Error unmarshalling algorithm definition: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error parsing request")
	}
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlModelConfig.MLAlgorithm,
		fmt.Sprintf("Update model config %s", mlModelConfig.Name),
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
		r.service.LoggingClient().
			Errorf("Invalid parameter 'sampleSize'. Error while parsing: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Invalid parameter 'sampleSize'")
	}

	generatePreview := c.QueryParam("generatePreview")
	r.service.LoggingClient().Infof("Value of generatePreview queryParameter: %v", generatePreview)
	// Perform validation based on the MLModelConfig struct tags
	err = r.validate.Struct(mlModelConfig)
	if err != nil {
		r.service.LoggingClient().Errorf("Error validating ML model configuration: %v", err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Error validating ML model configuration",
		)
	}

	_, hErr := r.mlModelConfigService.GetMLModelConfig(
		mlModelConfig.MLAlgorithm,
		mlModelConfig.Name,
	)
	if hErr != nil && hErr.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		r.service.LoggingClient().
			Errorf("Unable to get ML model config, config %s not found", mlModelConfig.MLAlgorithm)
		return hErr.ConvertToHTTPError()
	}

	err = r.mlModelConfigService.ValidateMLModelConfigFilters(
		mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters,
	)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Error validating filters in ML model configuration: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf("Invalid filters for model configuration, error: %v", err),
		)
	}
	algorithm, err := r.mlModelConfigService.GetAlgorithm(mlModelConfig.MLAlgorithm)
	if err != nil || algorithm == nil {
		return echo.NewHTTPError(
			http.StatusNotFound,
			"Failed to save ML model config "+mlModelConfig.Name+": algorithm "+mlModelConfig.MLAlgorithm+" not found.",
		)
	}
	mlModelConfig.MLAlgorithmType = algorithm.Type
	// Build the feature index in here
	r.mlModelConfigService.BuildFeatureNameToIndexMap(
		&mlModelConfig,
		algorithm.TimeSeriesAttributeRequired,
		algorithm.GroupByAttributesRequired,
	)
	err = r.mlModelConfigService.ValidateFeaturesToProfileByAlgoType(&mlModelConfig, true)
	if err != nil {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to save model config "+mlModelConfig.Name+" for algorithm "+mlModelConfig.MLAlgorithm+": error: "+err.Error(),
		)
	}

	err = r.mlModelConfigService.SaveMLModelConfig(&mlModelConfig, algorithm)

	// Asynchronous call for recreating training data sample
	if generatePreview == "true" || generatePreview == "yes" {
		go r.trainingDataService.CreateTrainingSampleAsync(&mlModelConfig, sampleSizeInt)
	}

	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to save ML model config for algo %s, model config %s. Error: %v", mlModelConfig.MLAlgorithm, mlModelConfig.Name, err)
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Failed to save model config "+mlModelConfig.Name+" for algorithm "+mlModelConfig.MLAlgorithm,
		)
	} else {
		hedgeErr = r.trainingDataService.SetTrainingDataExportDeprecated(&mlModelConfig)
		if hedgeErr != nil && hedgeErr.ErrorType() != hedgeErrors.ErrorTypeNotFound {
			r.service.LoggingClient().Errorf("Failed to set data export deprecated for algorithm: %s, mlModelConfig: %s. Error: %s", mlModelConfig.MLAlgorithm, mlModelConfig.Name, hedgeErr.Message())
		}
		_ = c.NoContent(http.StatusCreated)
	}
	return nil
}

func (r *Router) getModelConfiguration(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().
		Info("GET request for MLModel configuration")
	// alternative to edgexSdk.LoggingClient.Info("TEST")
	mlAlgorithm := c.Param("algorithmName")
	mlModelConfigName := c.Param("mlModelConfigName")

	summaryOnlyStr := c.QueryParam("summaryOnly")
	summaryOnly := false
	if summaryOnlyStr != "" {
		var err error
		summaryOnly, err = strconv.ParseBool(summaryOnlyStr)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Invalid parameter summaryOnly. Error while parsing: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid parameter summaryOnly")
		}
	}
	var trainingConfig interface{}
	var err hedgeErrors.HedgeError
	if mlAlgorithm == "all" || mlAlgorithm == "All" || mlModelConfigName == "all" ||
		mlModelConfigName == "All" {
		trainingConfig, err = r.getModelConfigs(mlAlgorithm, mlModelConfigName, summaryOnly)
	} else {
		trainingConfig, err = r.getModelConfig(mlAlgorithm, mlModelConfigName, summaryOnly)
	}
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to get model config %s for algo %s. Error: %v", mlModelConfigName, mlAlgorithm, err)
		return err.ConvertToHTTPError()
	} else {
		_ = c.JSON(http.StatusOK, trainingConfig)
	}
	return nil
}

func (r *Router) deleteModelConfiguration(c echo.Context) *echo.HTTPError {
	mlAlgorithm := c.Param("algorithmName")
	mlModelConfigName := c.Param("mlModelConfigName")
	hedgeErr := r.validateMlAlgorithmEnabled(
		mlAlgorithm,
		fmt.Sprintf("Delete model config %s", mlModelConfigName),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	jobsInProgress, err := r.trainingJobService.GetJobsInProgress(mlAlgorithm, mlModelConfigName)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to delete ML model config for algo %s, model config %s. Failed to get jobs in progress. Error: %s",
				mlAlgorithm, mlModelConfigName, err.Error())
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Failed to delete model with name "+mlModelConfigName+" for algorithm "+mlAlgorithm,
		)
	}
	if len(jobsInProgress) != 0 {
		msg := fmt.Sprintf(
			"Unable to delete ML model config for algo %s, model config %s. There are %d jobs in progress",
			mlAlgorithm,
			mlModelConfigName,
			len(jobsInProgress),
		)
		r.service.LoggingClient().Error(msg)
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}
	err = r.mlModelConfigService.DeleteMLModelConfig(mlAlgorithm, mlModelConfigName)

	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to delete ML model config for algo %s, model config %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
		return err.ConvertToHTTPError()
	} else {
		_ = c.NoContent(http.StatusOK)
	}

	return nil
}

func (r *Router) getPredictionTemplateByType(c echo.Context) *echo.HTTPError {
	t := c.QueryParam("algorithmType")
	if t == "" {
		err := hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"Required query parameter 'algorithmType' is missing",
		)
		return err.ConvertToHTTPError()
	}

	template, typeFound := config2.DefaultInputPayloadTemplatesPerAlgoType[t]
	if !typeFound {
		err := hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"Platform do not support template type: "+t,
		)
		return err.ConvertToHTTPError()
	}

	err := c.JSON(http.StatusOK, template)
	if err != nil {
		err := hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to encode template",
		)
		return err.ConvertToHTTPError()
	}
	return nil
}

func (r *Router) getAlgorithmTypes(c echo.Context) *echo.HTTPError {
	algoTypes := config2.GetAlgorithmTypes()
	err := c.JSON(http.StatusOK, algoTypes)
	if err != nil {
		hedgeErr := hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to get algorithm types",
		)
		return hedgeErr.ConvertToHTTPError()
	}
	return nil
}

func (r *Router) setAlgoTypeToModelConfig(
	algo *config.MLAlgorithmDefinition,
	modelConfig *config.MLModelConfig,
) (*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	// no need to set algo type, it is already set
	if modelConfig.MLAlgorithmType != "" {
		return algo, nil
	}
	if algo == nil {
		var err hedgeErrors.HedgeError
		algo, err = r.mlModelConfigService.GetAlgorithm(modelConfig.MLAlgorithm)
		if err != nil {
			return nil, err
		}
	}
	modelConfig.MLAlgorithmType = algo.Type
	return algo, nil
}

func (r *Router) getModelConfigsPerAlgo(
	mlAlgorithm string,
	mlModelConfigName string,
) ([]config.MLModelConfig, hedgeErrors.HedgeError) {
	var modelConfigs []config.MLModelConfig
	var err hedgeErrors.HedgeError
	if mlModelConfigName == "all" || mlModelConfigName == "All" {
		modelConfigs, err = r.mlModelConfigService.GetAllMLModelConfigs(mlAlgorithm)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to fetch model config definitions for algo %s. Error: %v", mlAlgorithm, err)
			return nil, err
		}
	} else {
		modelConfig, err := r.mlModelConfigService.GetMLModelConfig(mlAlgorithm, mlModelConfigName)
		if err != nil {
			r.service.LoggingClient().Errorf("Failed to fetch model config %s for algo %s. Error: %v", mlModelConfigName, mlAlgorithm, err)
			return nil, err
		}
		modelConfigs = append(modelConfigs, *modelConfig)
	}
	var algo *config.MLAlgorithmDefinition
	for i := range modelConfigs {
		algo, err = r.setAlgoTypeToModelConfig(algo, &modelConfigs[i])
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to set algo type to model config %s, algo %s. Error: %v", modelConfigs[i].Name, mlAlgorithm, err)
			return nil, err
		}
	}
	return modelConfigs, nil
}

func (r *Router) getModelConfig(
	mlAlgorithm string,
	mlModelConfigName string,
	summaryOnly bool,
) (interface{}, hedgeErrors.HedgeError) {
	modelConfig, err := r.mlModelConfigService.GetMLModelConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to fetch model config definition for algo %s, modelConfigName %s. Error: %v", mlAlgorithm, mlModelConfigName, err)
		return nil, err
	}
	_, err = r.setAlgoTypeToModelConfig(nil, modelConfig)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to set algo type to model config %s, algo %s. Error: %v", modelConfig.Name, mlAlgorithm, err)
		return nil, err
	}
	if !summaryOnly {
		return modelConfig, nil
	}
	summary, err := r.createModelConfigSummary(modelConfig)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to create summary for algo %s, modelConfigName %s. Error: %v", modelConfig.MLAlgorithm, modelConfig.Name, err)
		return nil, err
	}
	return summary, nil
}

func (r *Router) getModelConfigs(
	mlAlgorithm string,
	mlModelConfigName string,
	summaryOnly bool,
) (interface{}, hedgeErrors.HedgeError) {
	var modelConfigs []config.MLModelConfig

	if mlAlgorithm == "all" || mlAlgorithm == "All" {
		algos, err := r.mlModelConfigService.GetAllAlgorithms()
		if err != nil {
			r.service.LoggingClient().Errorf("Failed to fetch algo definitions. Error: %v", err)
			return nil, err
		}
		for _, algo := range algos {
			modelConfigsPerAlgo, err := r.getModelConfigsPerAlgo(algo.Name, mlModelConfigName)
			if err != nil {
				if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
					continue
				}
				r.service.LoggingClient().
					Errorf("Failed to fetch model config definitions for algo %s. Error: %v", algo.Name, err)
				return nil, err
			}
			modelConfigs = append(modelConfigs, modelConfigsPerAlgo...)
		}
	} else {
		modelConfigsPerAlgo, err := r.getModelConfigsPerAlgo(mlAlgorithm, mlModelConfigName)
		if err != nil {
			r.service.LoggingClient().Errorf("Failed to fetch model config definitions for algo %s. Error: %v", mlAlgorithm, err)
			return nil, err
		}
		modelConfigs = append(modelConfigs, modelConfigsPerAlgo...)
	}
	if !summaryOnly {
		return modelConfigs, nil
	}
	var summaries []*config.MLModelConfigSummary
	for _, modelConfig := range modelConfigs {
		summary, err := r.createModelConfigSummary(&modelConfig)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to create summary for modelConfig %s. Error: %v", modelConfig.Name, err)
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (r *Router) createModelConfigSummary(
	modelConfig *config.MLModelConfig,
) (*config.MLModelConfigSummary, hedgeErrors.HedgeError) {
	deployments, err := r.trainingJobService.GetModelDeploymentsByConfig(
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		false,
	)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to get model deploymets status for algo %s, modelConfigName %s. Error: %v", modelConfig.MLAlgorithm, modelConfig.Name, err)
		return nil, err
	}
	summary := config.MLModelConfigSummary{}
	summary.Name = modelConfig.Name
	summary.Description = modelConfig.Description
	summary.MLAlgorithm = modelConfig.MLAlgorithm
	summary.Enabled = modelConfig.Enabled
	summary.MLAlgorithmType = modelConfig.MLAlgorithmType
	summary.Deployments = make(map[string]int)
	for _, deployment := range deployments {
		if count, exists := summary.Deployments[deployment.DeploymentStatus]; exists {
			summary.Deployments[deployment.DeploymentStatus] = count + 1
		} else {
			summary.Deployments[deployment.DeploymentStatus] = 1
		}
	}
	latestTrainingJob, err := r.trainingJobService.GetLatestJobSummary(
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"",
	)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to get the latest job summary for algo %s, modelConfigName %s. Error: %v", modelConfig.MLAlgorithm, modelConfig.Name, err)
		return nil, err
	}
	if latestTrainingJob != nil {
		summary.TrainingJobStatus = latestTrainingJob.Status
	}
	return &summary, nil
}

func (r *Router) getAlgo(
	ctx context.Context,
	name string,
	summaryOnly bool,
) (interface{}, hedgeErrors.HedgeError) {
	if name == "all" || name == "All" {
		return r.getAllAlgos(ctx, summaryOnly)
	}
	return r.getSingleAlgo(ctx, name, summaryOnly)
}

// getAllAlgos fetches all algorithms and validates them.
func (r *Router) getAllAlgos(
	ctx context.Context,
	summaryOnly bool,
) (interface{}, hedgeErrors.HedgeError) {
	algos, err := r.mlModelConfigService.GetAllAlgorithms()
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to fetch algo definitions. Error: %v", err)
		return nil, err
	}

	// Validate image digests for each algo
	// Parallelized approach with fail-fast mechanism (because image digest validations takes ~3s per algo (2 images))
	r.service.LoggingClient().Info("Validating images for algos...")
	updatedAlgos, err := r.validateAlgorithmsInParallel(ctx, algos)
	if err != nil {
		return nil, err
	}

	if summaryOnly {
		r.service.LoggingClient().Info("Creating summaries for algos...")
		algoSummaries, err := r.createSummaries(ctx, updatedAlgos)
		if err != nil {
			return nil, err
		}
		return algoSummaries, nil
	}
	return updatedAlgos, nil
}

// validateAlgorithmsInParallel validates algorithms in parallel and collects the results.
func (r *Router) validateAlgorithmsInParallel(
	ctx context.Context,
	algos []*config.MLAlgorithmDefinition,
) ([]*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	var updatedAlgos []*config.MLAlgorithmDefinition
	var mu sync.Mutex
	g, cancelCtx := errgroup.WithContext(ctx)

	for _, algo := range algos {
		nextAlgo := algo
		g.Go(func() error {
			r.mlModelConfigService.ValidateMLImagesForGetAlgoFlow(cancelCtx, nextAlgo)
			mu.Lock()
			updatedAlgos = append(updatedAlgos, nextAlgo)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		var hedgeErr hedgeErrors.HedgeError
		if errors.As(err, &hedgeErr) {
			return nil, hedgeErr
		}
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	return updatedAlgos, nil
}

// createSummaries creates summaries for algorithms in parallel.
func (r *Router) createSummaries(
	ctx context.Context,
	algos []*config.MLAlgorithmDefinition,
) ([]*config.MLAlgorithmSummary, hedgeErrors.HedgeError) {
	var summaries []*config.MLAlgorithmSummary
	var mu sync.Mutex
	g, cancelCtx := errgroup.WithContext(ctx)

	for _, algo := range algos {
		nextAlgo := algo
		g.Go(func() error {
			select {
			case <-cancelCtx.Done():
				return cancelCtx.Err()
			default:
				summary, err := r.createAlgoSummary(nextAlgo)
				if err != nil {
					return err
				}
				mu.Lock()
				summaries = append(summaries, summary)
				mu.Unlock()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		var hedgeErr hedgeErrors.HedgeError
		if errors.As(err, &hedgeErr) {
			return nil, hedgeErr
		}
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	return summaries, nil
}

// getSingleAlgo fetches a single algorithm and validates it.
func (r *Router) getSingleAlgo(
	ctx context.Context,
	name string,
	summaryOnly bool,
) (interface{}, hedgeErrors.HedgeError) {
	algo, err := r.mlModelConfigService.GetAlgorithm(name)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to fetch algo definition for %s. Error: %v", name, err)
		return nil, err
	}
	if algo == nil {
		errMsg := fmt.Sprintf(
			"Failed to fetch algo definition for %s. Error: not found in db",
			name,
		)
		r.service.LoggingClient().Errorf(errMsg)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errMsg)
	}

	r.mlModelConfigService.ValidateMLImagesForGetAlgoFlow(ctx, algo)

	if summaryOnly {
		return r.createAlgoSummary(algo)
	}
	return algo, nil
}

func (r *Router) createAlgoSummary(
	algoDef *config.MLAlgorithmDefinition,
) (*config.MLAlgorithmSummary, hedgeErrors.HedgeError) {
	if algoDef == nil {
		return nil, nil
	}

	modelConfigs, err := r.mlModelConfigService.GetAllMLModelConfigs(algoDef.Name)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Error creating algo summary for algo %s. Error: %v", algoDef.Name, err)
		return nil, err
	}
	summary := config.MLAlgorithmSummary{}
	summary.Name = algoDef.Name
	summary.Description = algoDef.Description
	summary.Type = algoDef.Type
	summary.Enabled = algoDef.Enabled
	summary.ConfiguredModelCounts = int64(len(modelConfigs))
	summary.IsOotb = algoDef.IsOotb
	summary.MLDigestsValid = true

	if algoDef.DeprecatedTrainerImageDigest != "" || algoDef.DeprecatedPredictionImageDigest != "" {
		summary.MLDigestsValid = false
		summary.ErrorMessage = "The docker image has been updated without approval, so new training and deployment will fail"
	}
	return &summary, nil
}
