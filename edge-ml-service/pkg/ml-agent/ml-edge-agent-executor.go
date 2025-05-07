/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package ml_agent

import (
	"hedge/edge-ml-service/pkg/docker"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"hedge/common/client"
	"hedge/common/config"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/db/redis"
	DTO "hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/edge-ml-service/pkg/helpers"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
)

var HttpClient client.HTTPClient
var MlStorage helpers.MLStorageInterface

type MLEdgeAgentConfigInterface interface {
	LoadConfigurations(service interfaces.ApplicationService)
	DownloadModel(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	DeployInferenceImplementation(
		ctx interfaces.AppFunctionContext,
		data interface{},
	) (bool, interface{})
	SetHttpSender(mlBrokerUrl string)
	ExecuteMLCommands(
		ctx interfaces.AppFunctionContext,
		params interface{},
	) (continuePipeline bool, result interface{})
	GetReinitializeEndpoint() string
	GetModelDownloadEndpoint() string
	GetModelDeployTopic() string
	GetModelDownloadTopic() string
	SetDbClient(service interfaces.ApplicationService)
}

type MLEdgeAgentConfig struct {
	// To read model
	ModelDeployTopic      string
	LocalMLModelDir       string
	ModelDownloadTopic    string
	RemoteNodeId          string
	PredictEndpoint       string
	ReinitializeEndpoint  string
	ModelDownloadEndpoint string
	ModelDownloadEnabled  bool
	MqttUserName          string
	MqttPassword          string
	MqttScheme            string
	MqttServer            string
	MqttPort              int64
	QoS                   byte
	InsecureSkipVerify    bool
	RegistryConfig        helpers.ImageRegistryConfigInterface
	// whether to expose Prediction container ports or not, expose only for dev/qa enns
	ExposePredictContainerPort bool
	Node                       string
	Host                       string
	Subscriber                 MLAgentMQTTPublisherInterface
	HttpSender                 service.HTTPSenderInterface
	DbClient                   redis.MLDbInterface
	MlStorage                  helpers.MLStorageInterface
	ContainerManager           docker.ContainerManager
}

func NewMLEdgeAgentConfig(
	service interfaces.ApplicationService,
) (MLEdgeAgentConfigInterface, error) {
	mlEdgeAgentConfig := new(MLEdgeAgentConfig)
	mlEdgeAgentConfig.LoadConfigurations(service)

	registryCreds := mlEdgeAgentConfig.RegistryConfig.GetRegistryCredentials()
	if registryCreds.UserName == "" || registryCreds.Password == "" ||
		registryCreds.RegistryURL == "" {
		return nil, fmt.Errorf("image registry credentials not found")
	}

	manager, err := docker.NewDockerContainerManager(
		service,
		mlEdgeAgentConfig.LocalMLModelDir,
		registryCreds.RegistryURL,
		registryCreds.UserName,
		registryCreds.Password,
		mlEdgeAgentConfig.ExposePredictContainerPort,
	)
	if err != nil {
		return nil, err
	}

	mlEdgeAgentConfig.ContainerManager = manager
	return mlEdgeAgentConfig, nil
}

func (mlCfg *MLEdgeAgentConfig) SetDbClient(service interfaces.ApplicationService) {
	if mlCfg.DbClient == nil {
		dbConfig := db.NewDatabaseConfig()
		dbConfig.LoadAppConfigurations(service)
		mlCfg.DbClient = redis.NewDBClient(dbConfig)
	}
}

func (mlCfg *MLEdgeAgentConfig) LoadConfigurations(service interfaces.ApplicationService) {

	lc := service.LoggingClient()
	modelDeployTopic, err := service.GetAppSetting("ModelDeployTopic")
	if err != nil {
		lc.Errorf("Error reading the configuration for ModelDeployTopic: %s", err.Error())
	}

	modelDir, err := service.GetAppSetting("LocalMLModelDir")
	if err != nil {
		lc.Errorf("Error reading the configuration for LocalMLModelDir: %s", err.Error())
	} else if !strings.HasPrefix(modelDir, "/") {
		modelDir, _ = filepath.Abs(modelDir)

	}

	modelDownloadTopic, err := service.GetAppSetting("ModelDownloadTopic")
	if err != nil {
		lc.Errorf("Error reading the configuration for ModelDownloadTopic: %s", err.Error())
	}

	ReinitializeEndpoint, err := service.GetAppSetting("ReinitializeEndpoint")
	if err != nil {
		lc.Errorf("Error reading the configuration for ReinitializeEndpoint: %s", err.Error())
	} else {
		mlCfg.ReinitializeEndpoint = ReinitializeEndpoint
	}

	ModelDownloadEndpoint, err := service.GetAppSetting("ModelDownloadEndpoint")
	if err != nil {
		lc.Errorf("Error reading the configuration for ModelDownloadEndpoint: %s", err.Error())
	} else {
		mlCfg.ModelDownloadEndpoint = ModelDownloadEndpoint
	}

	mqttServer, err := service.GetAppSetting("MqttServer")
	if err != nil {
		lc.Errorf("Error reading the configuration for MqttServer: %s", err.Error())
	} else {
		lc.Infof("MqttServer : %s\n", mqttServer)
		mlCfg.MqttServer = mqttServer
	}

	mqttScheme, err := service.GetAppSetting("MqttScheme")
	if err != nil {
		mqttScheme = "tcp"
	}
	mlCfg.MqttScheme = mqttScheme
	mqttUserName, _ := service.GetAppSetting("MqttUserName")
	mqttPassword, _ := service.GetAppSetting("MqttPassword")
	mlCfg.MqttUserName = mqttUserName
	mlCfg.MqttPassword = mqttPassword

	mlCfg.ModelDeployTopic = modelDeployTopic

	mlCfg.QoS = config.GetMQTTQoS(service)

	mqttPortStr, err := service.GetAppSetting("MqttPort")
	if err != nil {
		lc.Errorf("Error reading the configuration for MqttPort: %s", err.Error())
	} else {
		lc.Infof("MqttPort : %v", mqttPortStr)
		mlCfg.MqttPort, _ = strconv.ParseInt(mqttPortStr, 0, 64)
	}

	mlCfg.ModelDownloadEnabled = true

	mlCfg.LocalMLModelDir = modelDir
	mlCfg.ModelDownloadTopic = modelDownloadTopic

	// Nodeid of the remote core server, this is required to download the model via nats proxy
	remoteNodeId, err := service.GetAppSetting("Remote_Node_Id")
	if err != nil {
		lc.Warnf("Error reading the configuration for Remote_Node_Id: %s", err.Error())
	}
	if remoteNodeId == "" {
		lc.Warnf(
			"Remote_Node_Id cannot be empty, please set this to core node id ( not host) and restart",
		)
		mlCfg.RemoteNodeId = ""
		mlCfg.ModelDownloadEndpoint = strings.Replace(
			mlCfg.ModelDownloadEndpoint,
			"nats-proxy:48200",
			"hedge-ml-management:48095",
			1,
		)
	} else {
		mlCfg.RemoteNodeId = remoteNodeId
	}
	lc.Infof("Remote_Node_Id: %s", remoteNodeId)
	lc.Infof("DownloadEndPoint: %s", mlCfg.ModelDownloadEndpoint)

	// Below should not be required with NATS now, will delete retained for now in case nats has issue
	insecureSkipVerify, err := service.GetAppSetting("InsecureSkipVerify")
	if err != nil {
		lc.Errorf("Error reading the configuration for InsecureSkipVerify: %s", err.Error())
	}
	if insecureSkipVerify == "true" {
		mlCfg.InsecureSkipVerify = true
	} else {
		mlCfg.InsecureSkipVerify = false
	}

	mlCfg.RegistryConfig = helpers.NewImageRegistryConfig(service)
	mlCfg.RegistryConfig.LoadRegistryConfig()

	exposePredictContainerPortStr, err := service.GetAppSetting("ExposePredictContainerPort")
	mlCfg.ExposePredictContainerPort = false
	if err != nil {
		lc.Warnf("Error reading the configuration for ExposeContainerPort: %s", err.Error())
	} else {
		lc.Infof("ExposeContainerPort set to: %s", exposePredictContainerPortStr)
		exposePredictContainerPort, err := strconv.ParseBool(exposePredictContainerPortStr)
		if err == nil {
			mlCfg.ExposePredictContainerPort = exposePredictContainerPort
		}
	}

	mlCfg.Subscriber = NewMLModelReadySubscriber(service, mlCfg)
}

func (mlCfg *MLEdgeAgentConfig) DownloadModel(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	lc := ctx.LoggingClient()

	if _, ok := data.(ml_model.ModelDeploymentStatus); ok {
		ctx.LoggingClient().Errorf("error in pipeline, so propagating")
		return true, data
	}

	modelDeployCommand, ok := data.(ml_model.ModelDeployCommand)
	if !ok {
		return false, fmt.Errorf(
			"function ReadFromTopicAndDownloadModel in pipeline '%s', type received is not a ModelDeployCommand",
			ctx.PipelineId(),
		)
	}

	lc.Infof("model details received: %v", modelDeployCommand)

	// If command is to update MLEvent config, just update and don't ack back
	if modelDeployCommand.CommandName == "SyncMLEventConfig" ||
		strings.ToLower(modelDeployCommand.CommandName) == "undeploy" {
		// no additional communication with ml-mgmt required, next pipeline step will handle the command
		return true, modelDeployCommand
	}

	downloadModelUrlBase := mlCfg.ModelDownloadEndpoint + "?mlModelConfigName=" + modelDeployCommand.MLModelConfigName + "&mlAlgorithm=" + modelDeployCommand.MLAlgorithm

	if HttpClient == nil {
		if mlCfg.InsecureSkipVerify {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // for Dev/Test purpose
			}
			HttpClient = &http.Client{Transport: tr}
		} else {
			HttpClient = &http.Client{}
		}
	}

	var chunks [][]byte
	var totalChunks int

	for i := 0; ; i++ { // Keep polling until all chunks received
		chunkUrl := fmt.Sprintf("%s&chunkIndex=%d", downloadModelUrlBase, i)
		lc.Infof("download model by chunks - URL=%s", chunkUrl)

		req, err := http.NewRequest("GET", chunkUrl, nil)
		if err != nil {
			lc.Errorf("error creating http request for url: %s, error: %v", chunkUrl, err)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				"error creating http client",
			)
		}
		// Add the headers to go via nats proxy
		if mlCfg.RemoteNodeId != "" {
			req.Header.Add("X-NODE-ID", mlCfg.RemoteNodeId)
			req.Header.Set("X-Forwarded-For", "hedge-ml-management:48095")
		}
		resp, err := HttpClient.Do(req)
		if err != nil {
			msg := fmt.Sprintf(
				"error While downloading model from url: %v, error: %v",
				chunkUrl,
				err,
			)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				msg,
			)
		}
		if resp.StatusCode > 200 {
			msg := fmt.Sprintf(
				"error status returned downloading the model to the edge, httpStatus: %d",
				resp.StatusCode,
			)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				msg,
			)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lc.Errorf("error reading chunk %d: %v", i, err)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				"error while reading model file chunk",
			)
		}
		resp.Body.Close()

		var chunkMsg ml_model.ChunkMessage
		err = json.Unmarshal(body, &chunkMsg)
		if err != nil {
			lc.Errorf("failed to parse chunk %d: %v", i, err)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				"error while parsing model file chunk",
			)
		}

		// First chunk received, store TotalChunks count
		if i == 0 {
			totalChunks = chunkMsg.TotalChunks
		}

		chunks = append(chunks, chunkMsg.ChunkData)
		if len(chunks) == totalChunks {
			break
		}

		// Check for missing chunks (eg ml-management restarted)
		if i < totalChunks-1 && chunkMsg.ChunkIndex != i {
			lc.Errorf("ml-management/nats-proxy restart detected, expected chunk %d but got %d", i, chunkMsg.ChunkIndex)
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				"error while getting model file chunk",
			)
		}
	}

	// Reassemble chunks
	savedModelData := bytes.Join(chunks, nil)
	if len(savedModelData) == 0 {
		lc.Errorf("Failed to assemble model data: No data received")
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			"error assembling model data",
		)
	}
	lc.Infof("Successfully reassembled model file data for ML model config %s, algorithm %s", modelDeployCommand.MLModelConfigName, modelDeployCommand.MLAlgorithm)

	if MlStorage == nil { // for test purpose only, for dev - new ml storage always created
		mlCfg.MlStorage = helpers.NewMLStorage(
			mlCfg.LocalMLModelDir,
			modelDeployCommand.MLAlgorithm,
			modelDeployCommand.MLModelConfigName,
			lc,
		)
	}
	err1 := mlCfg.MlStorage.AddModelAndConfigFile(savedModelData)
	if err1 != nil {
		msg := fmt.Sprintf("error downloading the model, error : %v", err1)
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	// Get the Version Number from config.json
	lc.Info("model successfully downloaded")
	// Delete .modelignore file if that exists ie if that got added during undeploy command
	mlCfg.MlStorage.RemoveModelIgnoreFile()
	return true, modelDeployCommand
}

func (mlCfg *MLEdgeAgentConfig) DeployInferenceImplementation(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	lc := ctx.LoggingClient()

	if _, ok := data.(ml_model.ModelDeploymentStatus); ok {
		lc.Errorf("error in pipeline, so propagating")
		return true, data
	}

	modelDeployCommand, ok := data.(ml_model.ModelDeployCommand)
	if !ok {
		lc.Errorf(
			"pipeline '%s', object type received is not valid deploy command",
			ctx.PipelineId(),
		)
		return false, nil
	}

	// no need to recreate inference service if ML event config has changed
	if modelDeployCommand.CommandName == "SyncMLEventConfig" {
		return true, modelDeployCommand
	}

	lc.Infof("algorithm deployment details received for: %s", modelDeployCommand.MLAlgorithm)

	image := modelDeployCommand.Algorithm.PredictionImagePath

	mlConfigWithAlgo, err := helpers.ReadMLConfigWithAlgo(
		mlCfg.LocalMLModelDir,
		lc,
		modelDeployCommand.MLAlgorithm,
	)
	if err != nil {
		msg := fmt.Sprintf(
			"error while reading config.json file of deployment, error: %s",
			err.Error(),
		)
		ctx.LoggingClient().Error(msg)
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	if _, ok := mlConfigWithAlgo[modelDeployCommand.MLModelConfigName]; !ok {
		msg := fmt.Sprintf(
			"inconsistent config.json file for deployment, expected mlConfigName not found: %s",
			modelDeployCommand.MLModelConfigName,
		)
		ctx.LoggingClient().Error(msg)
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}
	algorithmDefinition := mlConfigWithAlgo[modelDeployCommand.MLModelConfigName].MLAlgoDefinition
	if algorithmDefinition.Name == "" {
		msg := fmt.Sprintf(
			"error while getting algorithm definition for mlModelConfig %s",
			modelDeployCommand.MLModelConfigName,
		)
		if err != nil {
			ctx.LoggingClient().Error(err.Error())
		}
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)

	}
	// Build the container name based on the prediction URL user has provided
	containerName, portNo := GetContainerNameAndPortFromUrl(&algorithmDefinition)
	lc.Infof("containerName: %s, portNo:%d", containerName, portNo)
	// there can be only 1 deployment/containerName per algoName per node
	// If there are multiple mlModelConfigs for the same deployment, the container will get updated with new
	// image for all mlModelConfigs which was not intentional
	// We will leave with it for now it is rare that we will see a side-affect

	lc.Infof("removing inference container if exists: %s, %s", image, containerName)
	if err = mlCfg.ContainerManager.RemoveContainer(containerName); err != nil {
		msg := fmt.Sprintf("error while removing container %s", containerName)
		ctx.LoggingClient().Error(err.Error())
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	if strings.ToLower(modelDeployCommand.CommandName) == "undeploy" {
		if mlCfg.MlStorage == nil {
			mlCfg.MlStorage = helpers.NewMLStorage(
				mlCfg.LocalMLModelDir,
				modelDeployCommand.MLAlgorithm,
				modelDeployCommand.MLModelConfigName,
				lc,
			)
		}
		err = mlCfg.MlStorage.AddModelIgnoreFile()
		if err != nil {
			msg := "error while undeploying, modelignore file creation failed"
			return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
				ctx,
				modelDeployCommand,
				false,
				msg,
			)
		}
	}

	lc.Infof("Creating inference container: %s, %s:%d", image, containerName, portNo)
	id, err := mlCfg.ContainerManager.CreatePredicationContainer(
		image,
		algorithmDefinition.Name,
		portNo,
		containerName,
	)
	if err != nil {
		msg := fmt.Sprintf("error while creating container for %s", containerName)
		ctx.LoggingClient().Error(err.Error())
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	if err = mlCfg.ContainerManager.CopyModelToContainer(id, algorithmDefinition.Name); err != nil {
		msg := fmt.Sprintf(
			"failed copying model directory to inferencing container, error : %v",
			err,
		)
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	lc.Infof("Starting inference container: %s, %s", image, containerName)
	err = mlCfg.ContainerManager.StartContainer(containerName, id)
	if err != nil {
		msg := fmt.Sprintf("error while starting inference container for %s", containerName)
		ctx.LoggingClient().Error(err.Error())
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	ctx.LoggingClient().Infof("deployed algorithm %s inference container id: %v", containerName, id)
	return true, modelDeployCommand
}

func (mlCfg *MLEdgeAgentConfig) ExecuteMLCommands(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (continuePipeline bool, result interface{}) {

	if _, ok := data.(ml_model.ModelDeploymentStatus); ok {
		ctx.LoggingClient().Errorf("error in pipeline, so propagating")
		return true, data
	}
	modelDeployCommand, ok := data.(ml_model.ModelDeployCommand)

	if !ok {
		msg := "Technical error parsing model deploy command"
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	// Update Event threshold Config
	if strings.ToLower(modelDeployCommand.CommandName) != "undeploy" {
		if modelDeployCommand.CommandName == "SyncMLEventConfig" ||
			strings.ToLower(modelDeployCommand.CommandName) == "deploy" {
			err := mlCfg.SyncMLEventConfigs(modelDeployCommand, ctx.LoggingClient())
			if err != nil {
				ctx.LoggingClient().
					Errorf("error while saving ML event configuration: %s", err.Error())
				msg := fmt.Sprintf("error while saving ML event configuration, error: %v", err)
				return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
					ctx,
					modelDeployCommand,
					false,
					msg,
				)
			}
		}
	}

	// Notify broker to reload the model or the MLEvent config
	// broker internally notifies the inferencing containers since it knows them
	ctx.LoggingClient().
		Infof("sending HTTP Post to ML broker to reinitialize for %s model", modelDeployCommand.MLModelConfigName)

	ok, _ = mlCfg.HttpSender.HTTPPost(ctx, nil)
	ctx.LoggingClient().Info(fmt.Sprintf("HTTPS response for model deploy from broker: %v", ok))
	if !ok {
		msg := "error from API broker reinitialization"
		return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
			ctx,
			modelDeployCommand,
			false,
			msg,
		)
	}

	return true, mlCfg.Subscriber.BuildModelDownloadStatusPayload(
		ctx,
		modelDeployCommand,
		true,
		"Success",
	)
}

func (mlCfg *MLEdgeAgentConfig) SetHttpSender(mlBrokerUrl string) {
	cookie := service.NewDummyCookie()
	sender := service.NewHTTPSender(
		mlBrokerUrl,
		"application/json",
		false,
		make(map[string]string),
		&cookie,
	)
	mlCfg.HttpSender = sender
}

func (mlCfg *MLEdgeAgentConfig) GetReinitializeEndpoint() string {
	return mlCfg.ReinitializeEndpoint
}

func (mlCfg *MLEdgeAgentConfig) GetModelDownloadEndpoint() string {
	return mlCfg.ModelDownloadEndpoint
}

func (mlCfg *MLEdgeAgentConfig) GetModelDeployTopic() string {
	return mlCfg.ModelDeployTopic
}

func (mlCfg *MLEdgeAgentConfig) GetModelDownloadTopic() string {
	return mlCfg.ModelDownloadTopic
}

func (mlCfg *MLEdgeAgentConfig) SyncMLEventConfigs(
	command ml_model.ModelDeployCommand,
	lc logger.LoggingClient,
) error {
	eventConfigs := command.MLEventConfigsToSync
	for _, eventConfigToSync := range eventConfigs {
		switch eventConfigToSync.SyncCommand {
		case ml_model.CREATE:
			lc.Infof("Creating ML event config %s ...", eventConfigToSync.MLEventConfig.EventName)
			_, err := mlCfg.DbClient.AddMLEventConfig(eventConfigToSync.MLEventConfig)
			if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeConflict) {
				_, err = mlCfg.DbClient.UpdateMLEventConfig(
					eventConfigToSync.MLEventConfig,
					eventConfigToSync.MLEventConfig,
				)
			}
			return err
		case ml_model.UPDATE:
			oldEventConfigToUpdate := eventConfigToSync.OldMLEventConfig
			existingEventConfig, err := mlCfg.DbClient.GetMLEventConfigByName(
				oldEventConfigToUpdate.MLAlgorithm,
				oldEventConfigToUpdate.MlModelConfigName,
				oldEventConfigToUpdate.EventName,
			)
			if err != nil {
				if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
					lc.Infof(
						"ML event config %s doesn't exist on the node, creating it...",
						oldEventConfigToUpdate.EventName,
					)
					_, err = mlCfg.DbClient.AddMLEventConfig(eventConfigToSync.MLEventConfig)
					if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeConflict) {
						_, err = mlCfg.DbClient.UpdateMLEventConfig(
							eventConfigToSync.MLEventConfig,
							eventConfigToSync.MLEventConfig,
						)
					}
				}
				return err
			} else {
				lc.Infof("ML event config %s exists on the node, updating it...", existingEventConfig.EventName)
				_, err = mlCfg.DbClient.UpdateMLEventConfig(existingEventConfig, eventConfigToSync.MLEventConfig)
			}
			return err
		case ml_model.DELETE:
			eventConfigToDelete := eventConfigToSync.MLEventConfig
			err := mlCfg.DbClient.DeleteMLEventConfigByName(
				eventConfigToDelete.MLAlgorithm,
				eventConfigToDelete.MlModelConfigName,
				eventConfigToDelete.EventName,
			)
			return err
		default:
			lc.Infof(
				"Model command contains unrecognized SyncCommand for ML event configurations: %s",
				eventConfigToSync.SyncCommand,
			)
			return fmt.Errorf(
				"Model command contains unrecognized SyncCommand for ML event configurations: %s",
				eventConfigToSync.SyncCommand,
			)
		}
	}
	return nil
}

func GetContainerNameAndPortFromUrl(
	algorithmDefinition *DTO.MLAlgorithmDefinition,
) (string, int64) {

	// Very temporary for a day only so existing trained models continue to work
	// TODO: Girish remove this code below

	if !strings.HasPrefix(algorithmDefinition.DefaultPredictionEndpointURL, "http:") {
		return strings.ToLower(algorithmDefinition.Name), algorithmDefinition.DefaultPredictionPort
	}

	predictEndPointUrl := algorithmDefinition.DefaultPredictionEndpointURL
	// remove / and http or https from the URL
	// from http://multivariate-deepvar-timeseries:55000/api/v3/predict,
	// extract multivariate-deepvar-timeseries:55000
	container := ""
	tokens := strings.Split(predictEndPointUrl, "/")
	if len(tokens) < 2 {
		return container, 80
	}
	containerNPortTokens := strings.Split(tokens[2], ":")
	if strings.Contains(containerNPortTokens[0], ".") {
		container = strings.ToLower(algorithmDefinition.Name)
	} else {
		container = strings.ToLower(containerNPortTokens[0])
	}
	portNo, err := strconv.ParseInt(containerNPortTokens[1], 10, 0)
	if err == nil {
		return container, portNo
	} else {
		return container, 80
	}

}
