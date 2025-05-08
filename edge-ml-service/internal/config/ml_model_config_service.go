/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"golang.org/x/exp/maps"
	"hedge/edge-ml-service/pkg/dto/ml_model"

	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"hedge/common/service"
	"hedge/common/utils"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/edge-ml-service/pkg/db"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/helpers"
)

const (
	MLValidationFailedErr string = "ML model config validation failed: "
	PredictionImageType          = "Prediction"
	TrainerImageType             = "Trainer"
)

type MLModelConfigService struct {
	service      interfaces.ApplicationService
	mlMgmtConfig *config.MLMgmtConfig
	// dataSourceProvider service.DataStoreProvider
	dbClient          redis.MLDbInterface
	metricDataDb      db.MetricDataDbInterface
	httpClient        client.HTTPClient
	connectionHandler interface{} // Handles training provider connections
	registryConfig    helpers.ImageRegistryConfigInterface
}

func NewMlModelConfigService(
	service interfaces.ApplicationService,
	mlMgmtConfig *config.MLMgmtConfig,
	dbClient redis.MLDbInterface,
	dataSourceProvider service.DataStoreProvider,
	connectionHandler interface{},
	imageRegistryConfig helpers.ImageRegistryConfigInterface,
) *MLModelConfigService {
	mlModelConfigService := new(MLModelConfigService)
	mlModelConfigService.service = service
	mlModelConfigService.mlMgmtConfig = mlMgmtConfig
	mlModelConfigService.dbClient = dbClient
	mlModelConfigService.metricDataDb = db.NewMetricDataDb(
		service,
		mlMgmtConfig,
		dataSourceProvider,
	)
	mlModelConfigService.connectionHandler = connectionHandler
	mlModelConfigService.registryConfig = imageRegistryConfig
	return mlModelConfigService
}

// Below so that test stub can override with mock httpClient
func (s *MLModelConfigService) SetHttpClient() {
	s.httpClient = &http.Client{}
}

var DefaultInputPayloadTemplatesPerAlgoType = map[string]config.PredictionPayloadTemplate{
	helpers.ANOMALY_ALGO_TYPE: {
		Name:          helpers.ANOMALY_ALGO_TYPE,
		PayloadSample: helpers.ANOMALY_PAYLOAD_SAMPLE,
		Template:      helpers.ANOMALY_PAYLOAD_TEMPLATE,
	},
	helpers.CLASSIFICATION_ALGO_TYPE: {
		Name:          helpers.CLASSIFICATION_ALGO_TYPE,
		PayloadSample: helpers.CLASSIFICATION_PAYLOAD_SAMPLE,
		Template:      helpers.CLASSIFICATION_PAYLOAD_TEMPLATE,
	},
	helpers.TIMESERIES_ALGO_TYPE: {
		Name:          helpers.TIMESERIES_ALGO_TYPE,
		PayloadSample: helpers.TIMESERIES_PAYLOAD_SAMPLE,
		Template:      helpers.TIMESERIES_PAYLOAD_TEMPLATE,
	},
	helpers.REGRESSION_ALGO_TYPE: {
		Name:          helpers.REGRESSION_ALGO_TYPE,
		PayloadSample: helpers.REGRESSION_PAYLOAD_SAMPLE,
		Template:      helpers.REGRESSION_PAYLOAD_TEMPLATE,
	},
}

func (s *MLModelConfigService) GetRegistryConfig() helpers.ImageRegistryConfigInterface {
	return s.registryConfig
}

func GetAlgorithmTypes() []string {
	algoTypes := make([]string, 0, len(DefaultInputPayloadTemplatesPerAlgoType))
	for algoType := range DefaultInputPayloadTemplatesPerAlgoType {
		algoTypes = append(algoTypes, algoType)
	}
	return algoTypes
}

// GetAlgorithm returns the algorithm given a name
func (s *MLModelConfigService) GetAlgorithm(
	name string,
) (*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	return s.dbClient.GetAlgorithm(name)
}

// GetAllAlgorithms returns all algorithms
func (s *MLModelConfigService) GetAllAlgorithms() ([]*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	return s.dbClient.GetAllAlgorithms()
}

// CreateAlgorithm creates a new algorithm in the database
func (s *MLModelConfigService) CreateAlgorithm(
	algo config.MLAlgorithmDefinition,
	generateDigest bool,
	userId string,
) hedgeErrors.HedgeError {
	lc := s.service.LoggingClient()

	if algo.Name == helpers.DATA_COLLECTOR_NAME {
		lc.Warnf("Failed to create algorithm %s. The name is reserved", algo.Name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Algorithm name %s is reserved by system", algo.Name))
	}

	// Check for existing algorithm
	existingAlgo, err := s.dbClient.GetAlgorithm(algo.Name)
	if err != nil {
		lc.Errorf("Failed to get algo def from db %s. Error: %v", algo.Name, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			fmt.Sprintf("Failed to create algorithm %s", algo.Name),
		)
	}

	// return if an algorithm with the same name already exists
	if existingAlgo != nil {
		lc.Errorf("Algorithm %s already exists", algo.Name)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConflict,
			fmt.Sprintf("Algorithm %s already exists", algo.Name),
		)
	}

	// validate the algorithm and set defaults
	if hedgeError := s.validateAlgoAndSetDefaults(&algo); hedgeError != nil {
		return hedgeError
	}

	// Check whether training providers support the algorithm

	// Validate that training and prediction images exist in registry, set digest values
	err = s.validateMLImagesForCreateAlgoFlow(&algo, generateDigest)
	if err != nil {
		lc.Errorf(err.Error())
		return err
	} else {
		lc.Infof("Trainer image digest successfully set for algo %s: '%s'", algo.Name, algo.TrainerImageDigest)
		lc.Infof("Prediction image digest successfully set for algo %s: '%s'", algo.Name, algo.PredictionImageDigest)
	}

	if generateDigest {
		algo.LastApprovedBy = userId
	}

	// Create the algorithm and return an error if necessary
	err = s.dbClient.CreateAlgorithm(algo)
	if err != nil {
		lc.Errorf("Failed to add algo def to db %s. Error: %v", algo.Name, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			fmt.Sprintf("Failed to create algorithm %s", algo.Name),
		)
	}
	return nil
}

// validateAlgoAndSetDefaults is an eponymous function that validates the algorithm
func (s *MLModelConfigService) validateAlgoAndSetDefaults(
	algo *config.MLAlgorithmDefinition,
) hedgeErrors.HedgeError {

	// Assign default/derived values based on Algo type
	if algo.PredictionImagePath == "" {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeMandatory,
			"Prediction Image name is required",
		)
	}

	if !algo.IsTrainingExternal && algo.TrainerImagePath == "" {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeMandatory,
			"Trainer Image name is required",
		)
	}

	// set defaults for classification and regression
	switch algo.Type {
	case helpers.CLASSIFICATION_ALGO_TYPE, helpers.REGRESSION_ALGO_TYPE:
		algo.OutputFeaturesPredefined = true
	default:
		algo.OutputFeaturesPredefined = false
	}

	// set defaults for timeseries
	if algo.Type == helpers.TIMESERIES_ALGO_TYPE {
		algo.GroupByAttributesRequired = true
		algo.TimeSeriesAttributeRequired = true
	}

	// default values when deployment is allowed at edge
	if algo.AllowDeploymentAtEdge {
		algo.RequiresDataPipeline = true
		algo.IsPredictionRealTime = true
		algo.IsDeploymentExternal = false
	}

	// set payload template if not already set
	if algo.PredictionPayloadTemplate.Template == "" {
		algo.PredictionPayloadTemplate = DefaultInputPayloadTemplatesPerAlgoType[algo.Type]
	}

	// send default endpoint URL for prediction
	if algo.DefaultPredictionEndpointURL == "" {
		algo.DefaultPredictionEndpointURL = "/api/v3/predict"
	} else if strings.HasPrefix(algo.DefaultPredictionEndpointURL, "http") {
		// autofix the old prediction URL to align with what we want to keep
		// extract after first ":898989/"
		matchRegex, err := regexp.Compile("^https?://.*:[0-9]+/")
		if err == nil {
			algo.DefaultPredictionEndpointURL = matchRegex.ReplaceAllString(algo.DefaultPredictionEndpointURL, "/")
		}
	}
	if algo.DefaultPredictionPort == 0 {
		// Need to allocate one from non-system range: 49000 to 49900
		algo.DefaultPredictionPort = s.GetAllocatedPortNo()
	}

	// set PublishPredictionsRequired and AutoEventGenerationRequired true by default if not provided
	if algo.PublishPredictionsRequired == nil {
		defaultValue := true
		algo.PublishPredictionsRequired = &defaultValue
	}
	if algo.AutoEventGenerationRequired == nil {
		defaultValue := true
		algo.AutoEventGenerationRequired = &defaultValue
	}

	return nil
}

func (s *MLModelConfigService) GetAllocatedPortNo() int64 {
	algorithms, err := s.GetAllAlgorithms()
	if err == nil && algorithms != nil {
		portNo2Allocate := int64(49000)
		for _, algo := range algorithms {
			if portNo2Allocate <= algo.DefaultPredictionPort {
				portNo2Allocate = algo.DefaultPredictionPort + 1
			}
		}
		return portNo2Allocate
	} else {
		s.service.LoggingClient().Errorf("error getting algorithms")
		// Should not happen, so return 0 and let it be handled in next save of algo
		return 0
	}
}

// UpdateAlgorithm updates an algorithm in the database
func (s *MLModelConfigService) UpdateAlgorithm(
	algo config.MLAlgorithmDefinition,
	generateDigest bool,
	userId string,
) hedgeErrors.HedgeError {
	lc := s.service.LoggingClient()
	var msg string
	// Check if the algorithm exists
	existingAlgo, err := s.dbClient.GetAlgorithm(algo.Name)
	if err != nil {
		lc.Errorf(
			"Update algorithm %s failed. Error while getting existing algorithm: %v",
			algo.Name,
			err,
		)
		msg = fmt.Sprintf("Failed to update algorithm %s", algo.Name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}

	// Check whether the algorithm can be updated
	// 1. Return if existingAlgo is nil
	// Return if not
	if existingAlgo == nil {
		msg = fmt.Sprintf("Failed to update nonexistent algorithm %s", algo.Name)
		lc.Errorf(msg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, msg)
	}

	// we don't allow to change isOotb flag
	if algo.IsOotb != existingAlgo.IsOotb {
		lc.Warnf(
			"isOotb=%v of the received algo definition will be replaced with isOotb=%v of the existing one. Algo name: %s",
			algo.IsOotb,
			existingAlgo.IsOotb,
			algo.Name,
		)
		algo.IsOotb = existingAlgo.IsOotb
	}

	// Validate the algorithm and set defaults
	if hedgeError := s.validateAlgoAndSetDefaults(&algo); hedgeError != nil {
		return hedgeError
	}

	// 2. For out of box algorithms it is not allowed to update a training/prediction image name
	if existingAlgo.IsOotb {
		trainerImageTagChanged, err := imageTagChanged(
			existingAlgo.TrainerImagePath,
			algo.TrainerImagePath,
		)
		if err != nil {
			errMsg := fmt.Sprintf(
				"Failed to update the trainer image path of the out of the box algorithm %s: %s",
				existingAlgo.Name,
				err.Error(),
			)
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errMsg)
		}
		if trainerImageTagChanged {
			lc.Infof(
				"The trainer image path '%s' will be changed to '%s' for the out of the box algorithm %s",
				existingAlgo.TrainerImagePath,
				algo.TrainerImagePath,
				existingAlgo.Name,
			)
		}
		predictionImageTagChanged, err := imageTagChanged(
			existingAlgo.PredictionImagePath,
			algo.PredictionImagePath,
		)
		if err != nil {
			errMsg := fmt.Sprintf(
				"Failed to update the prediction image path of the out of the box algorithm %s: %s",
				existingAlgo.Name,
				err.Error(),
			)
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errMsg)
		}
		if predictionImageTagChanged {
			lc.Infof(
				"The prediction image path '%s' will be changed to '%s' for the out of the box algorithm %s",
				existingAlgo.PredictionImagePath,
				algo.PredictionImagePath,
				existingAlgo.Name,
			)
		}
	}

	// Validate that training and prediction images exist in corresponding registries, set updated digest values
	err = s.validateMLImagesForUpdateAlgoFlow(&algo, existingAlgo, generateDigest)
	if err != nil {
		lc.Errorf(err.Error())
		return err
	}

	if generateDigest {
		algo.LastApprovedBy = userId
	}

	// Update the algorithm in the database
	err = s.dbClient.UpdateAlgorithm(algo)
	if err != nil {
		lc.Errorf("Update algorithm %s failed. DB error: %v", algo.Name, err)
		msg = fmt.Sprintf("Failed to update algorithm %s", algo.Name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}
	return nil
}

// DeleteAlgorithm deletes the algorithm given an algorithm name from the database
func (s *MLModelConfigService) DeleteAlgorithm(name string) hedgeErrors.HedgeError {

	var msg string
	// Get the training algorithm
	algo, err := s.dbClient.GetAlgorithm(name)
	if err != nil {
		s.service.LoggingClient().
			Errorf("Delete algorithm %s failed. Error while getting algorithm: %v", name, err)
		msg = fmt.Sprintf("Failed to delete algorithm %s", name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}

	// Check whether the algorithm can be deleted
	// 1. Return if algorithm is nil
	if algo == nil {
		s.service.LoggingClient().Errorf("Delete algorithm %s failed. It doesn't exist", name)
		msg = fmt.Sprintf("Failed to delete non-existent algorithm %s", name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, msg)
	}

	// 2. It is not allowed to delete out of box algorithms
	if algo.IsOotb {
		s.service.LoggingClient().Errorf("Delete algorithm %s failed. It is out of the box", name)
		msg = fmt.Sprintf("It is not allowed to delete the out of the box algorithm %s", name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, msg)
	}

	// Get all model configs for algorithm name
	modelConfigs, err := s.GetAllMLModelConfigs(name)
	if err != nil {
		s.service.LoggingClient().
			Errorf("Delete algorithm %s failed. Error while getting model configs: %v", name, err)
		msg = fmt.Sprintf("Failed to delete algorithm %s", name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}

	// 3. If model configs exist, we cannot delete the algorithm
	if len(modelConfigs) > 0 {
		s.service.LoggingClient().
			Errorf("Delete algorithm %s failed. It has %d model configs.", name, len(modelConfigs))
		msg = fmt.Sprintf(
			"Can't delete algorithm %s. It has %d model configurations.",
			name,
			len(modelConfigs),
		)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}

	err = s.dbClient.DeleteAlgorithm(name)
	if err != nil {
		s.service.LoggingClient().Errorf("Delete algorithm %s failed. DB error: %v", name, err)
		msg = fmt.Sprintf("Failed to delete algorithm %s", name)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, msg)
	}

	return nil
}

// ChangeAlgorithmStatus changes the algorithm status in the database
func (s *MLModelConfigService) ChangeAlgorithmStatus(
	name string,
	isEnabled bool,
) hedgeErrors.HedgeError {

	enableDisableStr := "enable"
	if !isEnabled {
		enableDisableStr = "disable"
	}

	// Get the algorithm from the database
	algo, err := s.dbClient.GetAlgorithm(name)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			s.service.LoggingClient().
				Errorf("%s algorithm %s failed. It doesn't exist", enableDisableStr, name)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("Failed to %s non-existing algorithm %s", enableDisableStr, name),
			)
		}

		s.service.LoggingClient().
			Errorf("%s algorithm %s failed. Error while getting algorithm: %v", enableDisableStr, name, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			fmt.Sprintf("Failed to %s algorithm %s", enableDisableStr, name),
		)
	}

	// return an error if the algorithm is nil
	if algo == nil {
		s.service.LoggingClient().
			Errorf("%s algorithm %s failed. It doesn't exist", enableDisableStr, name)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			fmt.Sprintf("Failed to %s non-existing algorithm %s", enableDisableStr, name),
		)
	}

	// Change the algorithm status
	algo.Enabled = isEnabled
	err = s.dbClient.UpdateAlgorithm(*algo)
	if err != nil {
		s.service.LoggingClient().
			Errorf("%s algorithm %s failed. Error while updating algorithm: %v", enableDisableStr, algo.Name, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			fmt.Sprintf("Failed to %s algorithm %s", enableDisableStr, name),
		)
	}
	return nil
}

// GetMetric names returns metrics names
func (s *MLModelConfigService) GetMetricNames(label string) ([]interface{}, error) {
	labels, err := s.metricDataDb.GetTSLabels(label)
	if err == nil {
		var labelWithStatus map[string]interface{}
		err = json.Unmarshal(labels, &labelWithStatus)
		if err == nil {
			if labelWithStatus["data"] != nil {
				labelArrayInterface := labelWithStatus["data"].([]interface{})
				return labelArrayInterface, err
			}
		}
	}
	return nil, err
}

// getDigitalTwinDevices Gets the list of devices for a given digitalTwin
func (s *MLModelConfigService) getDigitalTwinDevices(
	digitalTwinName string,
) ([]string, hedgeErrors.HedgeError) {

	errorMessage := fmt.Sprintf("Error getting digital twin devices for %s", digitalTwinName)

	if s.mlMgmtConfig.DigitalTwinUrl == "" {
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeConfig,
			"missing DigitalTwinUrl configuration",
		)
	}

	// URL example: http://localhost:48090/api/v3/twin/definition/name/WindTurbineFarm/devices
	url := fmt.Sprintf(
		"%s/twin/definition/name/%s/devices",
		s.mlMgmtConfig.DigitalTwinUrl,
		digitalTwinName,
	)
	s.service.LoggingClient().Debugf("digitwin url: %s", url)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		s.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	resp, err := s.httpClient.Do(request)
	if err != nil || resp.StatusCode != http.StatusOK {
		s.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	// Read and return response
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	var devices []string
	err = json.Unmarshal(bodyBytes, &devices)
	s.service.LoggingClient().Debugf("devices for the twin: %s: %v", digitalTwinName, devices)

	if err != nil {
		s.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return devices, nil
}

// SaveMLModelConfig saves the MLModelConfig to the config service after performing validations
func (s *MLModelConfigService) SaveMLModelConfig(
	mlModelConfig *config.MLModelConfig, algorithm *config.MLAlgorithmDefinition,
) error {

	badDataCollection := mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec <= 0 ||
		mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs <= 0

	if badDataCollection {
		return errors.New("training duration & sampling interval need to be >= 0")
	}

	if _, exists := mlModelConfig.MLDataSourceConfig.FeaturesByProfile[""]; exists {
		return errors.New("profile name in featuresByProfile can't be empty")
	}

	// Add default value for sampling interval if not provided
	if mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs == 0 {
		// Should be the case for time-series prediction as well
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs =
			mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs
	}

	// entityName is the profile in this case
	if mlModelConfig.MLDataSourceConfig.PrimaryProfile == "" {
		for profile, _ := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
			mlModelConfig.MLDataSourceConfig.PrimaryProfile = profile
			break
		}
	}

	// Turn off sliding window for time-series prediction
	if algorithm.Type == helpers.TIMESERIES_ALGO_TYPE {
		mlModelConfig.MLDataSourceConfig.SupportSlidingWindow = false
	}

	switch mlModelConfig.MLDataSourceConfig.EntityType {
	case config.ENTITY_TYPE_DIGITAL_TWIN:
		// get the twin definition and get the list of devices in there that is added to overall filters and FeatureFilter
		devices, err := s.getDigitalTwinDevices(mlModelConfig.MLDataSourceConfig.EntityName)
		if err != nil {
			return err
		}
		deviceFilter := config.FilterDefinition{
			Label:    "deviceName",
			Operator: "CONTAINS",
			Value:    strings.Join(devices, ","),
		}
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.Filters =
			buildTwinFilter(
				mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.Filters,
				deviceFilter,
			)
		mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters =
			buildTwinFilter(
				mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters,
				deviceFilter,
			)
		// Go over devices and add as featureFilter

	default:
		// Set the EntityName and EntityType based on number of profiles when not a digital twin
		numFeatures := len(mlModelConfig.MLDataSourceConfig.FeaturesByProfile)
		switch {
		case numFeatures == 1:
			mlModelConfig.MLDataSourceConfig.EntityType = config.ENTITY_TYPE_PROFILE
			if mlModelConfig.MLDataSourceConfig.EntityName == "" {
				mlModelConfig.MLDataSourceConfig.EntityName = maps.Keys(mlModelConfig.MLDataSourceConfig.FeaturesByProfile)[0]
			}
		default:
			mlModelConfig.MLDataSourceConfig.EntityType = config.ENTITY_TYPE_DEVICELABELS
			// let entityName is this case be mlModelConfig, ideally need this be user provided name
			if mlModelConfig.MLDataSourceConfig.EntityName == "" {
				mlModelConfig.MLDataSourceConfig.EntityName = mlModelConfig.Name
			}

		}
	}

	mlModelConfig.Modified = time.Now().UnixMilli()
	_, err := s.dbClient.SaveMLModelConfig(*mlModelConfig)
	if err != nil {
		s.service.LoggingClient().Error(err.Error())
		return err
	}

	return nil
}

func buildTwinFilter(
	filters []config.FilterDefinition,
	deviceFilter config.FilterDefinition,
) []config.FilterDefinition {
	found := false
	if filters == nil {
		filters = make([]config.FilterDefinition, 0)
		filters = append(filters, deviceFilter)
		found = true
	} else {
		for i, filter := range filters {
			if filter.Label == "deviceName" {
				found = true
				// update existing filter as per current definition of twin
				filters[i] = deviceFilter
				break
			}
		}
	}
	if !found {
		filters = append(filters, deviceFilter)
	}
	return filters
}

func (s *MLModelConfigService) BuildFeatureNameToIndexMap(
	mlModelConfig *config.MLModelConfig,
	timeSeriesAttributeRequired bool,
	groupByAttributesRequired bool,
) {

	// First get rid of duplicate FeatureElements if any
	mergeDuplicateFeatures(mlModelConfig)

	// Create a list of features from the query
	featureUniqueNames := make([]string, 0)
	externalFeatureNames := make([]string, 0)
	outputFeatureNames := make([]string, 0)

	featuresToIndex := make(map[string]int)

	/*
		Commenting this out as it is unused as of now.
		if mlModelConfig.MLDataSourceConfig.EntityType == config.ENTITY_TYPE_DIGITAL_TWIN {
		}
	*/

	// Unique feature list
	// if GroupBy=Profile, features = [m1, m2,m3], During actual data collection, each datarow = (d1m1,d1m2,d1m3)
	// if GroupBy = Label, features = [m1p1, m2p2, m3p1], During data collection,
	//     each datarow = (1abelA m1p1, 1abelA m2p2, 1abelA m3p1)

	if len(mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys) == 0 {
		mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = append(
			mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys,
			"deviceName",
		)
	}

	// TS field is required by DS folks and also for TS prediction
	if timeSeriesAttributeRequired {
		featureUniqueNames = append(featureUniqueNames, config.TIMESTAMP_FEATURE_COL_NAME)
	}
	if groupByAttributesRequired {
		featureUniqueNames = append(
			featureUniqueNames,
			mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys...)
	}

	for profile, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {

		for _, feature := range features {
			featureName := helpers.BuildFeatureName(profile, feature.Name)
			if utils.Contains(featureUniqueNames, featureName) &&
				utils.Contains(externalFeatureNames, featureName) {
				continue
			}

			switch {
			//TODO: Girish, Should remove IsExternal from here
			case feature.FromExternalDataSource:
				externalFeatureNames = append(externalFeatureNames, featureName)
			case feature.IsOutput && mlModelConfig.MLAlgorithmType == helpers.REGRESSION_ALGO_TYPE:
				outputFeatureNames = append(outputFeatureNames, featureName)
			default:
				featureUniqueNames = append(featureUniqueNames, featureName)
			}
		}
	}
	if len(mlModelConfig.MLDataSourceConfig.ExternalFeatures) > 0 {
		for _, feature := range mlModelConfig.MLDataSourceConfig.ExternalFeatures {
			featureName := helpers.BuildFeatureName("external", feature.Name)
			if feature.IsOutput && (mlModelConfig.MLAlgorithmType == helpers.REGRESSION_ALGO_TYPE || mlModelConfig.MLAlgorithmType == helpers.CLASSIFICATION_ALGO_TYPE) {
				outputFeatureNames = append(outputFeatureNames, featureName)
			} else if feature.IsInput {
				featureUniqueNames = append(featureUniqueNames, featureName)
			}
		}
	}

	// Append external (output - for regression) features at the end of the featureUniqueNames slice
	featureUniqueNames = append(featureUniqueNames, externalFeatureNames...)
	featureUniqueNames = append(featureUniqueNames, outputFeatureNames...)

	for index, featureName := range featureUniqueNames {
		featuresToIndex[featureName] = index
	}
	mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex = featuresToIndex
}

func mergeDuplicateFeatures(mlModelConfig *config.MLModelConfig) {
	mergedFeatureByProfile := make(map[string][]config.Feature)
	for profile, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		uniqueFeatureNames := map[string]struct{}{}
		deleteIndexes := []int{}
		for i, feature := range features {
			if _, ok := uniqueFeatureNames[feature.Name]; !ok {
				uniqueFeatureNames[feature.Name] = struct{}{}
			} else {
				deleteIndexes = append(deleteIndexes, i)
			}
		}
		sort.Slice(deleteIndexes, func(i, j int) bool {
			return deleteIndexes[i] > deleteIndexes[j]
		})
		// now delete the duplicate element from the features slice
		mergedFeatureByProfile[profile] = features
		for _, v := range deleteIndexes {
			mergedFeatureByProfile[profile] = append(mergedFeatureByProfile[profile][:v], mergedFeatureByProfile[profile][v+1:]...)
		}
	}
	mlModelConfig.MLDataSourceConfig.FeaturesByProfile = mergedFeatureByProfile
}

// DeleteMLModelConfig given an algorithm name, delete a training configuration
func (s *MLModelConfigService) DeleteMLModelConfig(
	mlAlgorithm string,
	mlModelConfigName string,
) hedgeErrors.HedgeError {

	// Don't allow to delete if a trained model exist against this config
	modelDeployments, err := s.dbClient.GetDeploymentsByConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		s.service.LoggingClient().
			Errorf("Error getting deployments by ml model config name: %s: %v", mlModelConfigName, err)
		return hedgeErrors.NewCommonHedgeError(
			err.ErrorType(),
			fmt.Sprintf("Failed deleting ML model config: %s", mlModelConfigName),
		)
	}

	// Check for model deployments and return error if any of these are NOT in undeployed, ready to deploy or EOL status
	for _, modelDeployment := range modelDeployments {

		modelNotUndeployed := modelDeployment.DeploymentStatusCode != ml_model.ModelUnDeployed
		modelNotReadyToDeploy := modelDeployment.DeploymentStatusCode != ml_model.ReadyToDeploy
		modelNotEOL := modelDeployment.DeploymentStatusCode != ml_model.ModelEndOfLife

		if modelNotUndeployed && modelNotReadyToDeploy && modelNotEOL {
			errorMsg := fmt.Sprintf("There is a trained model in the deployed status, "+
				"so the ML model configuration cannot be deleted. "+
				"Please undeploy the model and then delete the ML model: %s",
				modelDeployment.ModelName,
			)
			s.service.LoggingClient().Error(errorMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, errorMsg)
		}
	}

	err = s.dbClient.DeleteMLModelConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed deleting ML model config: %s", mlModelConfigName)
		s.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errorMsg)
	}

	// Remove all existing event configs for the model config
	mlEventConfigs, err := s.dbClient.GetAllMLEventConfigsByConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Failed getting all existing ML event configs for the ML model config: %s",
			mlModelConfigName,
		)
		s.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errorMsg)
	}
	for _, mlEventConfig := range mlEventConfigs {
		err = s.dbClient.DeleteMLEventConfigByName(
			mlAlgorithm,
			mlModelConfigName,
			mlEventConfig.EventName,
		)
		if err != nil {
			errorMsg := fmt.Sprintf(
				"Failed deleting existing ML event config %s for ML model config: %s",
				mlEventConfig.EventName,
				mlModelConfigName,
			)
			s.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
			return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errorMsg)
		}
	}

	// Delete all undeployed trained models associated with the config
	err = s.dbClient.DeleteMLModelsByConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		errorMsg := fmt.Sprintf(
			"Failed deleting all undeployed trained models associated with the ML model config: %s",
			mlModelConfigName,
		)
		s.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
		return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errorMsg)
	}

	modelConfigFolder := filepath.Join(
		s.mlMgmtConfig.BaseTrainingDataLocalDir,
		mlAlgorithm,
		mlModelConfigName,
	)
	osErr := os.RemoveAll(modelConfigFolder)
	if osErr != nil {
		s.service.LoggingClient().
			Warnf("Failed deleting model config folder %s. Error: %s", modelConfigFolder, osErr.Error())
	}

	/*
		TODO: under discussion
		 Get all training jobs for the ML model config and remove them
		 trainingJobs, err := cfgSvc.dbClient.GetMLTrainingJobsByConfig(mlModelConfigName, "")
		if err != nil {
			errorMsg := fmt.Sprintf("failed while getting all training jobs for the ML model config: %s", mlModelConfigName)
			cfgSvc.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
			return errors.New(errorMsg)
		}
		for _, job := range trainingJobs {
			err = cfgSvc.dbClient.DeleteMLTrainingJobs(job.Name)
			if err != nil {
				errorMsg := fmt.Sprintf("failed while deleting all training jobs for the ML model config: %s", mlModelConfigName)
				cfgSvc.service.LoggingClient().Errorf("%s, error: %s", errorMsg, err.Error())
				return errors.New(errorMsg)
			}
		}
	*/

	return nil
}

// GetMLModelConfig gets the model configuration
func (s *MLModelConfigService) GetMLModelConfig(
	mlAlgorithm string,
	mlModelConfigName string,
) (*config.MLModelConfig, hedgeErrors.HedgeError) {
	var mlModelConfig config.MLModelConfig
	var err hedgeErrors.HedgeError

	mlModelConfig, err = s.dbClient.GetMlModelConfig(mlAlgorithm, mlModelConfigName)
	if err != nil {
		s.service.LoggingClient().Errorf(err.Error())
		return nil, err
	}

	// Update trainingConfig with Algo inferencingEndPoint and trainingProvider if they are empty
	algoConfig, err := s.GetAlgorithm(mlAlgorithm)
	if err == nil {
		if mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL == "" {
			mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL =
				fmt.Sprintf(
					"%s/%s/%s",
					algoConfig.BuildDefaultPredictionEndPointUrl(),
					algoConfig.Name,
					mlModelConfig.Name,
				)
		}
	}

	// Auto-create Input Topic for inferencing
	if mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName == "" {
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName = "enriched/events/device/"
		if len(mlModelConfig.MLDataSourceConfig.FeaturesByProfile) == 1 {
			// Get the profile name
			for profileName := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
				mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName += profileName + "/#"
			}
		} else {
			mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName += "#"
		}
	}

	return &mlModelConfig, nil
}

// GetAllMLModelConfigs gets all MLModelConfigs
func (s *MLModelConfigService) GetAllMLModelConfigs(
	mlAlgorithm string,
) ([]config.MLModelConfig, hedgeErrors.HedgeError) {
	mlModelConfigs, err := s.dbClient.GetAllMLModelConfigs(mlAlgorithm)
	if err != nil {
		s.service.LoggingClient().Error(err.Error())
		return mlModelConfigs, err
	}

	return mlModelConfigs, nil
}

// ValidateMLModelConfigFilters validates model config filters
func (s *MLModelConfigService) ValidateMLModelConfigFilters(
	filters []config.FilterDefinition,
) error {
	labelOperatorCount := make(map[string]map[string]int)

	for _, filter := range filters {
		// Initialize operator count map for the label if not already done:
		if _, exists := labelOperatorCount[filter.Label]; !exists {
			labelOperatorCount[filter.Label] = make(map[string]int)
		}

		// Increment the count for this operator and label:
		labelOperatorCount[filter.Label][filter.Operator]++

		// Get the counts for each operator
		containsCount := labelOperatorCount[filter.Label]["CONTAINS"]
		excludeCount := labelOperatorCount[filter.Label]["EXCLUDES"]
		otherCount := 0

		// Count all other operators:
		for op, count := range labelOperatorCount[filter.Label] {
			if op != "CONTAINS" && op != "EXCLUDES" {
				otherCount += count
			}
		}

		// Validate the count of operators for the label:
		if (containsCount > 1 || excludeCount > 1) ||
			(otherCount > 0 && (containsCount > 0 || excludeCount > 0)) {
			return fmt.Errorf(
				"invalid filter combination for attribute: %s, only 'CONTAINS & EXCLUDES' filters combination is allowed in case of addition of more than 1 filter per attribute",
				filter.Label,
			)
		}
	}
	return nil
}

// ValidateFeaturesToProfileByAlgoType validates features by algorithm type
func (s *MLModelConfigService) ValidateFeaturesToProfileByAlgoType(
	mlModelConfig *config.MLModelConfig,
	setAllFeaturesIsOutput bool,
) error {
	// Validation 1: For 1-profile case - only 1 element allowed for GroupOrJoinKeys - 'deviceName'
	if len(mlModelConfig.MLDataSourceConfig.FeaturesByProfile) == 1 {
		if len(mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys) != 1 ||
			strings.ToLower(mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys[0]) != "devicename" {
			errMsg := fmt.Sprintf(
				MLValidationFailedErr + "for 1 profile scenario there must be only 1 element of 'groupOrJoinKeys' - 'deviceName'",
			)
			s.service.LoggingClient().Error(errMsg)
			return errors.New(errMsg)
		}
	}
	// Validation 2: dataCollectionTotalDurationSec/samplingIntervalSecs must be >= 100 for proper training
	if mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs == 0 {
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs = mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs
	}
	if mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec/mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs < 100 {
		errMsg := fmt.Sprintf(
			MLValidationFailedErr + "dataCollectionTotalDurationSec/samplingIntervalSecs - minimum 100 data points required for training",
		)
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	switch mlModelConfig.MLAlgorithmType {
	case helpers.ANOMALY_ALGO_TYPE:
		err := s.applyAnomalyAlgoTypeValidations(mlModelConfig)
		if err != nil {
			return err
		}
	case helpers.CLASSIFICATION_ALGO_TYPE:
		err := s.applyClassificationAlgoTypeValidations(mlModelConfig)
		if err != nil {
			return err
		}
	case helpers.TIMESERIES_ALGO_TYPE:
		err := s.applyTimeseriesAlgoTypeValidations(mlModelConfig, setAllFeaturesIsOutput)
		if err != nil {
			return err
		}
	case helpers.REGRESSION_ALGO_TYPE:
		err := s.applyRegressionAlgoTypeValidations(mlModelConfig)
		if err != nil {
			return err
		}
	default:
		errMsg := fmt.Sprintf(
			MLValidationFailedErr+"unsupported MLAlgorithmType: %s", mlModelConfig.MLAlgorithmType)
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

func (s *MLModelConfigService) applyAnomalyAlgoTypeValidations(
	mlModelConfig *config.MLModelConfig,
) error {

	for profileName, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		for _, feature := range features {
			// Validation 1: All features must be `isInput == true` and `isOutput == false` and `isExternal == false`
			if !feature.IsInput || feature.IsOutput {
				errMsg := fmt.Sprintf(
					MLValidationFailedErr+
						"feature '%s' in profile '%s' must be isInput=true and isOutput=false for Anomaly algorithm type",
					feature.Name, profileName)
				s.service.LoggingClient().Error(errMsg)
				return errors.New(errMsg)
			}
		}
	}

	return nil
}

// applyClassificationAlgoTypeValidations performs the validations for classifications
func (s *MLModelConfigService) applyClassificationAlgoTypeValidations(
	mlModelConfig *config.MLModelConfig,
) error {

	_, outputFeatureCount, commonInputOutputFeatureCount := s.getInputAndOutputFeatureCounts(mlModelConfig)

	// Validation 1: Only one feature must be isOutput=true, isExternal=true, isInput=false
	if outputFeatureCount > 1 {
		errMsg := MLValidationFailedErr +
			"multiple output features found, when only 1 output feature is allowed for classification algorithm type as of now"
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	// Validation 3: Ensure one output feature exists
	if outputFeatureCount == 0 {
		errMsg := MLValidationFailedErr + "no output feature defined for classification"
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	if commonInputOutputFeatureCount > 0 {
		errMsg := fmt.Sprintf("%s %d feature(s) is configured both input and output", MLValidationFailedErr, commonInputOutputFeatureCount)
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	// Validation 4: Ensure the output feature has the last index in FeatureNameToColumnIndex
	// We ensure this when we buildFeatureToIndex, so not needed
	return nil
}

// applyTimeseriesAlgoTypeValidations performs the validations for time series
func (s *MLModelConfigService) applyTimeseriesAlgoTypeValidations(
	mlModelConfig *config.MLModelConfig,
	setAllFeaturesIsOutput bool,
) error {
	// Validate that input Context length is set
	if mlModelConfig.MLDataSourceConfig.InputContextCount == 0 {
		errMsg := fmt.Sprintf(
			MLValidationFailedErr + "Input sequence length for timeseries prediction needs to be greater than 0",
		)
		s.service.LoggingClient().Error(errMsg)
		return errors.New(errMsg)
	}
	if mlModelConfig.MLDataSourceConfig.OutputPredictionCount == 0 {
		mlModelConfig.MLDataSourceConfig.OutputPredictionCount = mlModelConfig.MLDataSourceConfig.InputContextCount
	}

	if mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs < 60 ||
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs < 60 {
		errMsg := fmt.Sprintf(
			MLValidationFailedErr + "sampling interval for timeseries (training & prediction) needs to at least 1 min and multiples of 1 min",
		)
		s.service.LoggingClient().Error(errMsg)
		return errors.New(errMsg)
	}

	// Make the sampling interval a multiple of 60 secs, ensure predictionSamplingInterval same as trainingSamplingInterval
	mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs = int64(
		math.Round(
			float64(
				mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs,
			)/60.0,
		) * 60,
	)
	mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs = mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs
	// If dataCollectionTotalDurationSec/samplingIntervalSecs < 100 after samplingIntervalSecs adjustment - update dataCollectionTotalDurationSec accordingly
	if mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec/mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs < 100 {
		msg := fmt.Sprintf(
			MLValidationFailedErr + "dataCollectionTotalDurationSec/samplingIntervalSecs < 100 after samplingIntervalSecs adjustment - updating dataCollectionTotalDurationSec accordingly...",
		)
		s.service.LoggingClient().Debugf(msg)
		mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec = 100 * mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs
	}

	for profileName, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		_, outputFeatureCount, _ := s.getInputAndOutputFeatureCounts(mlModelConfig)
		atLeastOneOutput := outputFeatureCount > 0
		// Validation: If no features are isOutput==true while creating a new ML model config - set all features to isOutput==true (POST & PUT both),
		if !atLeastOneOutput {
			if setAllFeaturesIsOutput {
				s.service.LoggingClient().
					Infof("No features with isOutput=true found for profile '%s', setting all features to isOutput=true", profileName)
				for i := range features {
					mlModelConfig.MLDataSourceConfig.FeaturesByProfile[profileName][i].IsOutput = true
				}
			} else {
				errMsg := fmt.Sprintf(
					MLValidationFailedErr+"no features with isOutput=true found for profile '%s', must be at list 1 for Timeseries algo type",
					profileName)
				s.service.LoggingClient().Errorf(errMsg)
				return errors.New(errMsg)
			}
		}
	}
	return nil
}

// applyRegressionAlgoTypeValidations performs the validations for regressions
func (s *MLModelConfigService) applyRegressionAlgoTypeValidations(
	mlModelConfig *config.MLModelConfig,
) error {
	_, outputFeatureCount, commonInputOutputFeatureCount := s.getInputAndOutputFeatureCounts(mlModelConfig)

	// Validation 1: Only one feature must be isOutput=true
	if outputFeatureCount > 1 || outputFeatureCount == 0 {
		errMsg := fmt.Sprintf(
			MLValidationFailedErr+"only one output feature configuration required, found: %d", outputFeatureCount)
		//multiple output features found across profiles

		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}
	// Same feature cannot be both input and output for regression
	if commonInputOutputFeatureCount > 0 {
		errMsg := fmt.Sprintf("%s %d feature(s) is configured both input and output", MLValidationFailedErr, commonInputOutputFeatureCount)
		s.service.LoggingClient().Errorf(errMsg)
		return errors.New(errMsg)
	}

	// Validation 3: Ensure one output feature exists
	return nil
}

func (s *MLModelConfigService) validateMLImagesForCreateAlgoFlow(
	algo *config.MLAlgorithmDefinition,
	generateDigest bool,
) hedgeErrors.HedgeError {
	lc := s.service.LoggingClient()

	if (algo.TrainerImageDigest == "" || algo.PredictionImageDigest == "") && !generateDigest {
		errMsg := fmt.Sprintf(
			"image validation failed for algo %s: digest is not provided, generateDigest - %v",
			algo.Name,
			generateDigest,
		)
		lc.Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	g, _ := errgroup.WithContext(context.Background())

	// Validate trainer image in parallel
	g.Go(func() error {
		digest, err := s.registryConfig.GetImageDigest(algo.TrainerImagePath)
		errMsgBase := fmt.Sprintf(
			"%s image validation failed for algo %s, image '%s', generateDigest '%v",
			TrainerImageType,
			algo.Name,
			algo.TrainerImagePath,
			generateDigest,
		)
		if err != nil {
			if !generateDigest && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				errMsg := fmt.Sprintf("%s, err: %s", errMsgBase, err.Error())
				lc.Errorf(errMsg)
				return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errMsg)
			}
			return err
		}
		if !generateDigest && algo.TrainerImageDigest != digest {
			errMsg := fmt.Sprintf(
				"%s, err: provided digest is not found in image registry",
				errMsgBase,
			)
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errMsg)
		}
		algo.TrainerImageDigest = digest
		algo.DeprecatedTrainerImageDigest = ""
		return nil
	})

	// Validate prediction image in parallel
	g.Go(func() error {
		digest, err := s.registryConfig.GetImageDigest(algo.PredictionImagePath)
		errMsgBase := fmt.Sprintf(
			"%s image validation failed for algo %s, image '%s', generateDigest '%v",
			PredictionImageType,
			algo.Name,
			algo.PredictionImagePath,
			generateDigest,
		)
		if err != nil {
			if !generateDigest && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				errMsg := fmt.Sprintf("%s, err: %s", errMsgBase, err.Error())
				lc.Errorf(errMsg)
				return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errMsg)
			}
			return err
		}
		if !generateDigest && algo.PredictionImageDigest != digest {
			errMsg := fmt.Sprintf(
				"%s, err: provided digest is not found in image registry",
				errMsgBase,
			)
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errMsg)
		}
		algo.PredictionImageDigest = digest
		algo.DeprecatedPredictionImageDigest = ""
		return nil
	})

	if err := g.Wait(); err != nil {
		var hedgeErr hedgeErrors.HedgeError
		if errors.As(err, &hedgeErr) {
			return hedgeErr
		}
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}

	return nil
}

func (s *MLModelConfigService) validateMLImagesForUpdateAlgoFlow(
	updatedAlgo, existingAlgo *config.MLAlgorithmDefinition,
	generateDigest bool,
) hedgeErrors.HedgeError {
	lc := s.service.LoggingClient()

	// Validate if image paths have changed in update
	if !generateDigest &&
		(updatedAlgo.TrainerImagePath != existingAlgo.TrainerImagePath || updatedAlgo.PredictionImagePath != existingAlgo.PredictionImagePath) {
		errMsg := fmt.Sprintf(
			"Image validation failed: image path has been changed for algo %s, but generateDigest - %v",
			existingAlgo.Name,
			generateDigest,
		)
		lc.Errorf(errMsg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	validateAndSetDigest := func(imagePath, existingDigest, providedDigest, imageType string) (string, hedgeErrors.HedgeError) {
		// digest = Latest image digest from docker registry
		digest, err := s.registryConfig.GetImageDigest(imagePath)
		errMsgBase := fmt.Sprintf(
			"%s image validation failed for algo %s, image '%s', generateDigest '%v'",
			imageType,
			updatedAlgo.Name,
			imagePath,
			generateDigest,
		)
		if err != nil {
			errMsg := fmt.Sprintf("%s, err: %s", errMsgBase, err.Error())
			lc.Errorf(errMsg)
			return "", hedgeErrors.NewCommonHedgeError(err.ErrorType(), errMsg)
		}
		if !generateDigest && digest != existingDigest {
			errMsg := fmt.Sprintf(
				"%s, err: existing digest %s is not existing in image registry anymore",
				errMsgBase,
				existingDigest,
			)
			lc.Errorf(errMsg)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
		}
		if generateDigest && providedDigest == digest {
			lc.Infof("provided digest %s already upto date, no need to generate again",
				providedDigest)
		}
		return digest, nil
	}

	g, _ := errgroup.WithContext(context.Background())

	// Validate trainer image digest
	g.Go(func() error {
		digest, err := validateAndSetDigest(
			updatedAlgo.TrainerImagePath,
			existingAlgo.TrainerImageDigest,
			updatedAlgo.TrainerImageDigest,
			TrainerImageType,
		)
		if err != nil {
			return err
		}
		updatedAlgo.TrainerImageDigest = digest
		updatedAlgo.DeprecatedTrainerImageDigest = ""
		return nil
	})

	// Validate prediction image digest
	g.Go(func() error {
		digest, err := validateAndSetDigest(
			updatedAlgo.PredictionImagePath,
			existingAlgo.PredictionImageDigest,
			updatedAlgo.PredictionImageDigest,
			PredictionImageType,
		)
		if err != nil {
			return err
		}
		updatedAlgo.PredictionImageDigest = digest
		updatedAlgo.DeprecatedPredictionImageDigest = ""
		return nil
	})

	if err := g.Wait(); err != nil {
		var hedgeErr hedgeErrors.HedgeError
		if errors.As(err, &hedgeErr) {
			return hedgeErr
		}
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}

	return nil
}

func (s *MLModelConfigService) ValidateMLImagesForGetAlgoFlow(
	ctx context.Context,
	algo *config.MLAlgorithmDefinition,
) {
	lc := s.service.LoggingClient()
	imageMissingMsg := "image digest is missing in registry"
	serverErrorMsg := "image validation failed due to server error"
	var trainerImageDigest, predictionImageDigest string
	g, cancelCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-cancelCtx.Done():
			return cancelCtx.Err()
		default:
			digest, err := s.registryConfig.GetImageDigest(algo.TrainerImagePath)
			if err != nil {
				if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
					lc.Warnf(
						"Trainer image digest got deprecated for algo %s, no new digest found for the provided image path: %s",
						algo.Name,
						algo.TrainerImagePath,
					)
					algo.DeprecatedTrainerImageDigest = algo.TrainerImageDigest
					algo.TrainerImageDigest = imageMissingMsg
				} else {
					algo.DeprecatedTrainerImageDigest = serverErrorMsg
				}
				return err
			}
			trainerImageDigest = digest
			return nil
		}
	})

	g.Go(func() error {
		select {
		case <-cancelCtx.Done():
			return cancelCtx.Err()
		default:
			digest, err := s.registryConfig.GetImageDigest(algo.PredictionImagePath)
			if err != nil {
				if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
					lc.Warnf(
						"Prediction image digest got deprecated for algo %s, no new digest found for the provided image path: %s",
						algo.Name,
						algo.PredictionImagePath,
					)
					algo.DeprecatedPredictionImageDigest = algo.PredictionImageDigest
					algo.PredictionImageDigest = imageMissingMsg
				} else {
					algo.DeprecatedPredictionImageDigest = serverErrorMsg
				}
				return err
			}
			predictionImageDigest = digest
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		return
	}

	if trainerImageDigest != algo.TrainerImageDigest {
		lc.Warnf(
			"Trainer image digest %s got deprecated for algo %s, new digest - %s",
			algo.TrainerImageDigest,
			algo.Name,
			trainerImageDigest,
		)
		algo.DeprecatedTrainerImageDigest = algo.TrainerImageDigest
		algo.TrainerImageDigest = trainerImageDigest
	}
	if predictionImageDigest != algo.PredictionImageDigest {
		lc.Warnf(
			"Prediction image digest %s got deprecated for algo %s, new digest - %s",
			algo.PredictionImageDigest,
			algo.Name,
			predictionImageDigest,
		)
		algo.DeprecatedPredictionImageDigest = algo.PredictionImageDigest
		algo.PredictionImageDigest = predictionImageDigest
	}
}

func imageTagChanged(existingImage, updatedImage string) (bool, error) {
	existingParts := strings.Split(existingImage, ":")
	updatedParts := strings.Split(updatedImage, ":")

	// Validate both images have at least a name and a tag
	if len(existingParts) < 2 || len(updatedParts) < 2 {
		return false, fmt.Errorf(
			"invalid image format: both images must have a name and tag (e.g., 'nginx:1.21.0')",
		)
	}
	if existingParts[0] != updatedParts[0] {
		return false, fmt.Errorf("the image name is not allowed to be updated")
	}
	if existingParts[1] != updatedParts[1] {
		return true, nil
	}

	return false, nil
}

// Utility function to return number of input and output featureCounts. It is possible to have the same feature as input as well as output
func (s *MLModelConfigService) getInputAndOutputFeatureCounts(mlModelConfig *config.MLModelConfig) (inputCount int, outputCount int, commonInputOutputFeatureCount int) {
	inputFeatureCount, outputFeatureCount, commonInputOutputFeatureCount := 0, 0, 0
	for _, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		for _, feature := range features {
			// Validation 1: All features must be `isInput == true` and `isOutput == false`
			if feature.IsInput {
				inputFeatureCount++
				if feature.IsOutput {
					commonInputOutputFeatureCount++
				}
			}
			if feature.IsOutput {
				outputFeatureCount++
			}
		}
	}
	// We want to allow external features that can be input or output, just that they are provided outside of Hedge
	// What it means is that during training they can be uploaded as csv file, while during inferencing it can be provided as part of what-if condition
	if len(mlModelConfig.MLDataSourceConfig.ExternalFeatures) > 0 {
		for _, externalFeature := range mlModelConfig.MLDataSourceConfig.ExternalFeatures {
			if externalFeature.IsInput {
				inputFeatureCount++
			}
			if externalFeature.IsOutput {
				outputFeatureCount++
			}
		}
	}
	return inputFeatureCount, outputFeatureCount, commonInputOutputFeatureCount
}
