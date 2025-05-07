/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	redis2 "hedge/common/db/redis"
	commService "hedge/common/service"
	mlEdgeService "hedge/edge-ml-service/internal/config"
	"hedge/edge-ml-service/internal/training"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/helpers"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	bootstrapinterfaces "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	hedgeErrors "hedge/common/errors"
)

type Router struct {
	service              interfaces.ApplicationService
	appConfig            *config.MLMgmtConfig
	mlModelConfigService *mlEdgeService.MLModelConfigService
	trainingDataService  training.TrainingDataServiceInterface
	trainingJobService   *training.TrainingJobService
	telemetry            *helpers.Telemetry
	validate             *validator.Validate
}

const (
	ApplicationJson = "application/json"
	ContentType     = "Content-Type"
)

type RouteMap struct {
	Url     string
	Handler func(c echo.Context) *echo.HTTPError
	Label   string
	Method  string
}

func NewRouter(
	service interfaces.ApplicationService,
	appConfig *config.MLMgmtConfig,
	dbClient redis.MLDbInterface,
	serviceName string,
	metricsManager bootstrapinterfaces.MetricsManager,
) *Router {
	router := new(Router)
	router.service = service
	router.appConfig = appConfig

	router.telemetry = helpers.NewTelemetry(service, serviceName, metricsManager, dbClient)

	// We set the right ConnectionHandler and DataStoreProvider right here so that subsequent usage can be based on interface methods without explicit check of ADE vs local etc
	var connectionHandler interface{}
	var dataStoreProvider commService.DataStoreProvider

	connectionHandler = nil
	dataStoreProvider = commService.NewDefaultDataStoreProvider(appConfig.LocalDataStoreUrl)

	registryConfig := helpers.NewImageRegistryConfig(service)
	registryConfig.LoadRegistryConfig()

	router.mlModelConfigService = mlEdgeService.NewMlModelConfigService(
		service,
		appConfig,
		dbClient,
		dataStoreProvider,
		connectionHandler,
		registryConfig,
	)
	router.mlModelConfigService.SetHttpClient()

	router.trainingDataService = training.NewTrainingDataService(
		service,
		appConfig,
		dbClient,
		dataStoreProvider,
		commService.CommandRunner{},
		connectionHandler,
	)

	router.trainingJobService = training.NewTrainingJobService(
		service,
		appConfig,
		router.mlModelConfigService,
		dbClient,
		router.trainingDataService,
		router.telemetry,
		nil,
		connectionHandler,
	)
	router.trainingJobService.SetHttpClient()
	//router.trainingJobService.RegisterTrainingProviders()
	router.validate = validator.New()
	router.validate.RegisterValidation("matchRegex", matchRegex)
	// add separate validation for "EventMessage" field, because when a comma appears in the regex,
	// it is misinterpreted as the start of a new tag option, so can't be added directly in "validate" tag
	router.validate.RegisterValidation("eventMessageRegex", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[a-zA-Z0-9 .,;:_+=%{}()-]+$`)
		return regex.MatchString(fl.Field().String())
	})

	router.cleanupDataStatuses()

	return router
}

// AddRoutes adds routes to the service
func (r *Router) AddRoutes() {
	r.addMLRoutes()
}

func (r *Router) cleanupDataStatuses() {
	algos, err := r.mlModelConfigService.GetAllAlgorithms()
	if err != nil {
		r.service.LoggingClient().
			Errorf("cleanupDataStatuses won't be performed. Failed to get all algorithms. Error: %v", err)
		return
	}
	var modelConfigs []config.MLModelConfig
	for _, algo := range algos {
		modelConfigsPerAlgo, err := r.mlModelConfigService.GetAllMLModelConfigs(algo.Name)
		if err != nil {
			r.service.LoggingClient().
				Errorf("Failed to get model configs for algo %s. Their statuses won't be cleand up. Error: %v", algo.Name, err)
			continue
		}
		modelConfigs = append(modelConfigs, modelConfigsPerAlgo...)
	}
	r.trainingDataService.CleanupStatuses(modelConfigs)
}

func (r *Router) unmarshalToMLModelConfig(req *http.Request) (config.MLModelConfig, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return config.MLModelConfig{}, err
	}

	var mlModelConfig config.MLModelConfig

	if err := json.Unmarshal(body, &mlModelConfig); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return config.MLModelConfig{}, err

	}
	return mlModelConfig, nil
}

// Convert the content of full CSV File to JSON
func (r *Router) convCSVtoJSON(path string) ([]byte, string) {
	csvFile, err := os.Open(path)
	if err != nil {
		// log.Fatal("The file is not found || wrong root")
		return nil, "The file is not found"
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	content, _ := reader.ReadAll()

	if len(content) < 1 {
		return nil, "Something wrong, the file maybe empty or length of the lines are not the same"
	}

	headersArr := make([]string, 0)
	headersArr = append(headersArr, content[0]...)

	// Remove the header row
	content = content[1:]

	var buffer bytes.Buffer
	buffer.WriteString("[")
	for i, d := range content {
		buffer.WriteString("{")
		for j, y := range d {

			buffer.WriteString(`"` + headersArr[j] + `":`)
			_, fErr := strconv.ParseFloat(y, 32)
			_, bErr := strconv.ParseBool(y)

			switch {
			case fErr == nil:
				buffer.WriteString(y)
			case bErr == nil:
				buffer.WriteString(strings.ToLower(y))
			default:
				buffer.WriteString(`"` + y + `"`)
			}

			// end of property
			if j < len(d)-1 {
				buffer.WriteString(",")
			}

		}
		// end of object of the array
		buffer.WriteString("}")
		if i < len(content)-1 {
			buffer.WriteString(",")
		}
	}

	buffer.WriteString(`]`)
	rawMessage := json.RawMessage(buffer.String())
	x, err := json.MarshalIndent(rawMessage, "", "  ")
	if err != nil {
		return nil, err.Error()
	} else {
		return x, ""
	}
}

func matchRegex(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(fl.Param()) // Compile the regex pattern from the tag 'validate'
	return re.MatchString(fl.Field().String())
}

func resolveTrainingFilePath(
	c echo.Context,
	config *config.MLMgmtConfig,
	fileName string,
	suffix string,
) (string, error) {

	// Extract query parameters
	mlModelConfig := c.QueryParam("mlModelConfigName")
	mlAlgorithm := c.QueryParam("mlAlgorithm")
	baseFilePath := config.BaseTrainingDataLocalDir

	// Sanitize input paths
	cleanMlAlgorithm := filepath.Clean(mlAlgorithm)
	cleanTrainingConfig := filepath.Clean(mlModelConfig)

	// Prevent directory traversal
	if strings.Contains(cleanMlAlgorithm, "..") || strings.Contains(cleanTrainingConfig, "..") {
		return "", fmt.Errorf("invalid path component in mlAlgorithm or trainingConfig")
	}

	// Construct the relative path
	relativePath := filepath.Join(cleanMlAlgorithm, cleanTrainingConfig, suffix, fileName)
	fullPath := filepath.Join(baseFilePath, relativePath)

	// Ensure the path is within the base directory
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(baseFilePath)) {
		return "", fmt.Errorf("resolved file path is outside the base directory")
	}

	return fullPath, nil
}

// returnResponse encapsulates boilerplate code to send a response
func (r *Router) sendResponse(
	c echo.Context,
	status int,
	message []byte,
	headers map[string]string,
) error {
	logger := r.service.LoggingClient()
	response := c.Response()
	for key, value := range headers {
		response.Header().Set(key, value)
	}
	response.Write(message)
	response.WriteHeader(status)
	logger.Infof("response: %+v", response)

	return nil
}

func (r *Router) GetDbClient() redis2.CommonRedisDBInterface {
	return r.trainingJobService.GetDbClient()
}

func (r *Router) validateMlAlgorithmEnabled(
	mlAlgorithmName string,
	action string,
) hedgeErrors.HedgeError {
	r.service.LoggingClient().Infof("Validating whether algorithm %s is enabled", mlAlgorithmName)
	algo, err := r.mlModelConfigService.GetAlgorithm(mlAlgorithmName)
	if err != nil || algo == nil {
		r.service.LoggingClient().
			Errorf("%s failed. Failed to get algorithm %s. Error: %v", action, mlAlgorithmName, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s failed. Failed to get algorithm %s", action, mlAlgorithmName),
		)
	}
	if !algo.Enabled {
		msg := fmt.Sprintf("%s failed. Algorithm %s is disabled", action, mlAlgorithmName)
		r.service.LoggingClient().Warn(msg)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, msg)
	}
	return nil
}

func (r *Router) validateTrainerImageDigest(mlAlgorithmName string) hedgeErrors.HedgeError {
	lc := r.service.LoggingClient()
	algo, err := r.mlModelConfigService.GetAlgorithm(mlAlgorithmName)
	if err != nil || algo == nil {
		errMsg := fmt.Sprintf(
			"Failed to submit training job: error while getting algorithm %s.",
			mlAlgorithmName,
		)
		lc.Errorf(fmt.Sprintf("%s, err: %v", errMsg, err))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	if algo.TrainerImageDigest == "" {
		errMsg := fmt.Sprintf(
			"Failed to submit training job: trainer image digest is not set for algo %s",
			algo.Name,
		)
		lc.Errorf("%s, err %v", errMsg, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	} else {
		actualTrainerImageDigest, err := r.mlModelConfigService.GetRegistryConfig().GetImageDigest(algo.TrainerImagePath)
		lc.Infof("actualTrainerImageDigest - %s", actualTrainerImageDigest)
		errMsgBase := fmt.Sprintf("Failed to submit training job: trainer image validation failed for '%s', image digest '%s', algorithm '%s'", algo.TrainerImagePath, algo.TrainerImageDigest, algo.Name)
		if err != nil {
			errMsg := fmt.Sprintf("%s, err: %s", errMsgBase, err.Error())
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(err.ErrorType(), errMsg)
		}
		if actualTrainerImageDigest != algo.TrainerImageDigest {
			errMsg := fmt.Sprintf("%s, err: training image digest provided in algo definition doesn't exist in the image registry", errMsgBase)
			lc.Errorf(errMsg)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
		}
	}

	return nil
}
