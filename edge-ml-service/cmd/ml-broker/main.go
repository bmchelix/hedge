/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"hedge/common/client"
	commService "hedge/common/service"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/helpers"

	"hedge/edge-ml-service/internal/broker"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"

	"net/http"
)

var (
	serviceInt          interfaces.ApplicationService
	baseConfigDirectory string // Added since the current directory was getting changed after download and new configuration reads were failing
	Client              client.HTTPClient
	appServiceCreator   commService.AppServiceCreator
	osExit              = os.Exit
	chdirFunc           = os.Chdir
	initialEnvVars      map[string]string
)

func getAppService() {
	if appServiceCreator == nil {
		appServiceCreator = &commService.AppService{}
	}
	svc, ok := appServiceCreator.NewAppService(client.HedgeMLBrokerServiceKey)
	if !ok {
		err := fmt.Errorf("failed to start App Service: %s", client.HedgeMLBrokerServiceKey)
		fmt.Println(err)
		exitWrapper(-1)
	} else {
		serviceInt = svc
	}
}

func main() {
	captureInitialEnvVars()

	if serviceInt == nil {
		getAppService()
	}
	service := serviceInt
	lc := service.LoggingClient()

	currentDirectory, err := os.Getwd()
	if err != nil {
		lc.Errorf("Failed to get Project Directory: %v\n", err)
	}
	lc.Infof("Current Directory: %s", currentDirectory)
	baseConfigDirectory = currentDirectory

	Client = &http.Client{}

	mlBrokerConfig := broker.NewMLBrokerConfig()
	mlBrokerConfig.LoadConfigurations(service)

	notificationChannel := make(chan string)
	router := broker.NewRouter(service, mlBrokerConfig, notificationChannel)
	router.AddRoutes()

	err = BuildMLInferencePipelinesFromConfig(service, mlBrokerConfig, notificationChannel)
	if err != nil {
		lc.Errorf(
			"BuildMLInferencePipelinesFromConfig returned error, will continue, error : %v",
			err,
		)
	}
	// The below logic will be changed to subscribe to modelRegistration Subscription

	err = service.Run()
	if err != nil {
		lc.Errorf("Run returned error: %v", err)
		return
	}

	lc.Info("hedge ml-inferencing-broker terminating")
	exitWrapper(0)
}

// Reads the ml_model configuration from starting base ml_model directory and builds the pipeline with a ID
// The same AppFunctionPipeline cannot be started again using Run since it then throws port in use error
func BuildMLInferencePipelinesFromConfig(
	service interfaces.ApplicationService,
	appConfig *broker.MLBrokerConfig,
	notificationChannel <-chan string) error {

	lc := service.LoggingClient()

	currentDirectory, err := os.Getwd()
	if err != nil {
		lc.Warnf("%v", err)
	}

	if currentDirectory != baseConfigDirectory {
		err = chdirFunc(baseConfigDirectory)
		if err != nil {
			lc.Warnf("%v", err)
		}
	}
	lc.Info("New ML Broker Inference instance starting")
	configNameToConfigMap, err := helpers.ReadMLConfigWithAlgo(
		appConfig.LocalMLModelBaseDir,
		service.LoggingClient(),
		"",
	)
	if err != nil {
		service.LoggingClient().
			Errorf("Error scanning the model directory for comService and configuration, Error:%v", err)
		return err
	}

	//	closeInferPipelineCommandsByConfig = make(map[string]chan bool)
	// Now that we have the ml_model in the form of modelConfig,
	// create a pipeline to collect sample data and then predict
	// To work out how to handle multiple training configurations/comService in the same pipeline
	noOfModels := len(configNameToConfigMap)
	service.LoggingClient().Infof("configNameToConfigMap: %v", configNameToConfigMap)
	service.LoggingClient().Infof("No of Anomaly Models: %d\n", noOfModels)
	errCount := 0
	for configName, trainingConfig := range configNameToConfigMap {
		service.LoggingClient().
			Infof("Starting a new pipeline for configName: %s, trainingConfig: %+v", configName, trainingConfig)
		err := addMLInferencePipeline(
			service,
			configName,
			trainingConfig.MLModelConfig,
			trainingConfig.MLAlgoDefinition,
			appConfig,
			notificationChannel,
		)
		if err != nil {
			service.LoggingClient().
				Errorf("Error creating inference pipeline for training config: %s, error: %v", trainingConfig.MLModelConfig.Name, err)
			errCount++
		}
	}
	if len(configNameToConfigMap) > 0 && errCount == len(configNameToConfigMap) {
		// return error only if all configs result in error
		return errors.New("error adding new pipeline for the configurations")
	}

	service.LoggingClient().Info("ML Broker Inference App pipeline added")

	return nil
}

func addMLInferencePipeline(
	service interfaces.ApplicationService,
	configName string,
	mlModelConfig config.MLModelConfig,
	mlAlgoDefinition config.MLAlgorithmDefinition,
	mlBrokerCfg *broker.MLBrokerConfig,
	notificationChannel <-chan string,
) error {

	lc := service.LoggingClient()
	lc.Info("ml-broker-broker pipeline starting for config:" + configName)

	eventPipelineTriggerUrl := mlBrokerCfg.EventsPublisherURL
	// Below has to be taken from training config & then built by adding trainingdata config name

	basePredictUrl := mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL
	if basePredictUrl == "" || !strings.HasPrefix(basePredictUrl, "http") {
		basePredictUrl = mlAlgoDefinition.BuildDefaultPredictionEndPointUrl()
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL =
			fmt.Sprintf(
				"%s/%s/%s", basePredictUrl,
				mlAlgoDefinition.Name,
				mlModelConfig.Name,
			)
	}

	lc.Infof(
		"prediction endpoint: %s",
		mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL,
	)

	// Not sure if we really need this realtime this, will need to deprecate this
	isRealTime := true
	realTime, err := service.GetAppSetting("RealTimePredictions")
	if err == nil {
		isRealTime, _ = strconv.ParseBool(realTime)
	}

	var mlInf = broker.NewMLBrokerInference(
		&mlModelConfig,
		&mlAlgoDefinition,
		mlBrokerCfg,
		service,
		notificationChannel,
		isRealTime,
	)
	if mlInf == nil {
		return errors.New("Could not instantiate MlInferencing for training config:" + configName)
	}
	mlInf.SetHttpSender(eventPipelineTriggerUrl)
	topics := make([]string, 1)
	// TopicName is read from trainingConfig
	if mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName != "" {
		topics[0] = mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.TopicName
	} else {
		topics[0] = "enriched/events/device/#" // Build the topic name based on training data config, eg if groupByProfile, add profile to the topic
	}

	lc.Infof("Message Bus: %s", mlInf.ReadMessageBus)
	switch mlInf.ReadMessageBus {

	case "MQTT":
		if err := service.AddFunctionsPipelineForTopics(configName+":mqtt", topics,
			mlInf.FilterMetricsByFeatureNames,
			mlInf.TakeMetricSample,
			mlInf.GroupSamples,
			mlInf.ConvertToFeatures,
			mlInf.CollectAndReleaseFeature,
			mlInf.AccumulateMultipleTimeseriesFeaturesSets,
			mlInf.GetAndPublishPredictions,
			mlInf.PublishEvents,
		); err != nil {
			lc.Error(fmt.Sprintf("SDK SetPipeline failed: %v\n", err))
			return err
		}

	default:
		if err := service.AddFunctionsPipelineForTopics(configName+":redis", topics,
			// Take samples at regular intervals..
			// configPipeline.checkforNewModel,
			// transforms.NewFilterFor(profiles).FilterByProfileName,
			// Cannot use FilterByResource since it strips out Tags which we need
			// transforms.NewFilterFor(metrics).FilterByResourceName,
			// Add custom filter by groups if there are specific metadata grouping
			mlInf.FilterByFeatureNames,
			mlInf.TakeSample,
			mlInf.GroupSamples,
			mlInf.ConvertToFeatures,
			mlInf.CollectAndReleaseFeature,
			mlInf.AccumulateMultipleTimeseriesFeaturesSets,
			mlInf.GetAndPublishPredictions,
			mlInf.PublishEvents,
		); err != nil {
			lc.Error(fmt.Sprintf("SDK SetPipeline failed: %v\n", err))
			return err
		}

	}

	return nil
}

func captureInitialEnvVars() {
	env := os.Environ()
	initialEnvVars = make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			initialEnvVars[parts[0]] = parts[1]
		}
	}
}

func exitWrapper(code int) {
	osExit(code)
}
