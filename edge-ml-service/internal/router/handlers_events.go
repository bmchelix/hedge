package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	hedgeErrors "hedge/common/errors"
	mlConfig "hedge/edge-ml-service/pkg/dto/config"
	"github.com/labstack/echo/v4"
)

func (r *Router) addMLEventConfiguration(c echo.Context) *echo.HTTPError {
	var config mlConfig.MLEventConfig
	err := json.NewDecoder(c.Request().Body).Decode(&config)

	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to create MLEventConfig while decoding input payload: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to create MLEventConfig while decoding input payload",
		)
	}

	// Perform validation based on the MLEventConfig struct tags
	err = r.validate.Struct(config)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to create MLEventConfig while validating input payload: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to create MLEventConfig while validating input payload",
		)
	}
	r.service.LoggingClient().Info("Creating ML Event Config")

	algo, hErr := r.mlModelConfigService.GetAlgorithm(config.MLAlgorithm)
	if hErr != nil {
		r.service.LoggingClient().
			Errorf("Failed to create MLEventConfig for ML model %s while getting algorithm %s, error: %v", config.MlModelConfigName, config.MLAlgorithm, hErr)
		return hErr.ConvertToHTTPError()
	}
	config.MLAlgorithmType = algo.Type

	// Validate if ML model config exists
	_, hErr = r.mlModelConfigService.GetMLModelConfig(config.MLAlgorithm, config.MlModelConfigName)
	if hErr != nil {
		r.service.LoggingClient().
			Errorf("Failed to create MLEventConfig while getting ML model %s, error: %v", config.MlModelConfigName, hErr)
		return hErr.ConvertToHTTPError()
	}

	hErr = r.trainingJobService.AddMLEventConfig(config)
	if hErr != nil {
		if hErr.IsErrorType(hedgeErrors.ErrorTypeConflict) {
			r.service.LoggingClient().
				Errorf("Failed to create MLEventConfig for ML model %s, ML Event config with name %s already exists", config.MlModelConfigName, config.EventName)
			return hErr.ConvertToHTTPError()
		}

		if hErr.IsErrorType(hedgeErrors.MaxLimitExceeded) {
			r.service.LoggingClient().
				Errorf("Failed to create MLEventConfig for ML model %s, only 1 ML event config allowed for algorithm of type Anomaly", config.MlModelConfigName)
			return hErr.ConvertToHTTPError()
		}

		r.service.LoggingClient().
			Errorf("Failed to create MLEventConfig for ML model %s: %v", config.MlModelConfigName, hErr)
		return hErr.ConvertToHTTPError()
	}

	_ = c.JSON(http.StatusCreated, config)
	return nil
}

func (r *Router) updateMLEventConfiguration(c echo.Context) *echo.HTTPError {
	eventName := c.Param("eventName")
	if eventName == "" {
		err := errors.New("required parameter 'eventName' is missing")
		r.service.LoggingClient().
			Errorf("Failed to update MLEventConfig while decoding input payload: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to update MLEventConfig while decoding input payload, error: "+err.Error(),
		)
	}

	var config mlConfig.MLEventConfig
	err := json.NewDecoder(c.Request().Body).Decode(&config)

	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to update MLEventConfig while decoding input payload: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to update MLEventConfig while decoding input payload",
		)
	}
	// Perform validation based on the MLEventConfig struct tags
	err = r.validate.Struct(config)
	if err != nil {
		r.service.LoggingClient().
			Errorf("Failed to update MLEventConfig while validating input payload: %v", err)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Failed to update MLEventConfig while validating input payload",
		)
	}

	// Validate if ML model config still exists
	_, hErr := r.mlModelConfigService.GetMLModelConfig(config.MLAlgorithm, config.MlModelConfigName)
	if hErr != nil {
		r.service.LoggingClient().
			Errorf("Failed to update MLEventConfig while getting ML model %s, error: %v", config.MlModelConfigName, hErr)
		return hErr.ConvertToHTTPError()
	}

	existingMLEvent, hErr := r.trainingJobService.GetMLEventConfigByName(
		config.MLAlgorithm,
		config.MlModelConfigName,
		eventName,
	)
	if hErr != nil {
		if hErr.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			errMsg := fmt.Sprintf(
				"Failed to update MLEventConfig for ML model %s while getting existing ML event config: %v",
				config.MlModelConfigName,
				hErr,
			)
			r.service.LoggingClient().Errorf(errMsg)
			return echo.NewHTTPError(
				http.StatusNotFound,
				fmt.Sprintf("%s: %s", errMsg, "Item Not Found"),
			)
		} else {
			r.service.LoggingClient().Errorf("Failed to update MLEventConfig for ML model %s while getting existing ML event config: %v", config.MlModelConfigName, hErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update MLEventConfig for ML model "+config.MlModelConfigName+" while getting existing ML event config")
		}
	}

	config.MLAlgorithmType = existingMLEvent.MLAlgorithmType
	// Update Config in DB
	r.service.LoggingClient().Info("Updating ML event config")
	if err := r.trainingJobService.UpdateMLEventConfig(existingMLEvent, config); err != nil {
		r.service.LoggingClient().
			Errorf("Failed to update MLEventConfig for ML model %s: %s", config.MlModelConfigName, err.Error())
		return err.ConvertToHTTPError()
	}
	_ = c.NoContent(http.StatusCreated)
	return nil
}

func (r *Router) getMLEventConfiguration(c echo.Context) *echo.HTTPError {
	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	eventName := c.QueryParam("eventName")

	r.service.LoggingClient().
		Infof("Fetching config for Algorithm: %s, MLModelConfigName: %v, EventName: %s", mlAlgorithm, mlModelConfigName, eventName)

	config, err := r.trainingJobService.GetMLEventConfig(mlAlgorithm, mlModelConfigName, eventName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().
				Errorf("Failed to get MLEventConfig for ML model %s: %s", mlModelConfigName, err.Error())
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Failed to get MLEventConfig for ML model "+mlModelConfigName+", item not found",
			)
		} else {
			r.service.LoggingClient().Errorf("Failed to get MLEventConfig for ML model %s: %s", mlModelConfigName, err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get MLEventConfig for ML model "+mlModelConfigName)
		}
	}
	_ = c.JSON(http.StatusOK, config)
	return nil
}

func (r *Router) deleteMLEventConfiguration(c echo.Context) *echo.HTTPError {
	mlModelConfigName := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	eventName := c.QueryParam("eventName")
	r.service.LoggingClient().
		Infof("Deleting ML event config: %s, mlModelConfigName: %v, EventName: %s", mlAlgorithm, mlModelConfigName, eventName)

	err := r.trainingJobService.DeleteMLEventConfig(mlAlgorithm, mlModelConfigName, eventName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().
				Errorf("Failed to delete MLEventConfig for ML model %s: %s", mlModelConfigName, err.Error())
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Failed to delete MLEventConfig for ML model "+mlModelConfigName+", item not found",
			)
		} else {
			r.service.LoggingClient().Errorf("Failed to delete MLEventConfig for ML model %s: %s", mlModelConfigName, err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete MLEventConfig for ML model "+mlModelConfigName)
		}
	}
	_ = c.NoContent(http.StatusOK)

	return nil
}

func (r *Router) getTimeSeriesLabels(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().
		Info("Get call for metric labels")
		// alternative to edgexSdk.LoggingClient.Info("TEST")
	label := c.Param("label")
	if label == "" {
		r.service.LoggingClient().Errorf("Label is missing")
		return echo.NewHTTPError(http.StatusInternalServerError, "Label is missing")
	}
	labels, err := r.mlModelConfigService.GetMetricNames(label)
	if err != nil {
		r.service.LoggingClient().Errorf("Error getting metric labels by label %s: %v", label, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error getting metric labels")
	} else {
		_ = c.JSON(http.StatusOK, labels)
	}
	return nil
}
