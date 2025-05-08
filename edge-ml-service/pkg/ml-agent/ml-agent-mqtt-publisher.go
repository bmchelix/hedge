/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package ml_agent

import (
	"strings"
	"time"

	"hedge/edge-ml-service/pkg/dto/ml_model"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"

	"hedge/common/client"
	mqttConfig "hedge/common/config"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
)

type MqttSender interface {
	MQTTSend(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
}

type MLAgentMQTTPublisherInterface interface {
	GetMLAgentMQTTPublisher(
		service interfaces.ApplicationService,
		appConfig MLEdgeAgentConfigInterface,
	) MLAgentMQTTPublisherInterface
	BuildModelDownloadStatusPayload(
		ctx interfaces.AppFunctionContext,
		modelDeployCommand ml_model.ModelDeployCommand,
		isSuccess bool,
		message string,
	) ml_model.ModelDeploymentStatus
}

var MLAgentMQTTPublisherInterfaceImpl MLAgentMQTTPublisherInterface

type MLAgentMQTTPublisher struct {
	EdgexSdk   interfaces.ApplicationService
	AppConfig  MLEdgeAgentConfigInterface
	MqttSender MqttSender
}

func NewMLModelReadySubscriber(
	service interfaces.ApplicationService,
	appConfig *MLEdgeAgentConfig,
) MLAgentMQTTPublisherInterface {
	// For unit test case writing, we override the MLAgentMQTTPublisherInterfaceImpl
	if MLAgentMQTTPublisherInterfaceImpl == nil {
		MLAgentMQTTPublisherInterfaceImpl = &MLAgentMQTTPublisher{}
	}

	return MLAgentMQTTPublisherInterfaceImpl.GetMLAgentMQTTPublisher(service, appConfig)
}

func (agentPublisher *MLAgentMQTTPublisher) GetMLAgentMQTTPublisher(
	service interfaces.ApplicationService,
	appConfig MLEdgeAgentConfigInterface,
) MLAgentMQTTPublisherInterface {
	mlModelReadySubs := &MLAgentMQTTPublisher{
		AppConfig: appConfig,
		EdgexSdk:  service,
	}
	config, err := mqttConfig.BuildMQTTSecretConfig(
		service,
		appConfig.GetModelDownloadTopic(),
		client.HedgeMLEdgeAgentServiceKey,
	)
	if err != nil {
		service.LoggingClient().Errorf("Error Building MQTT client: Error: %v", err)
	} else {
		mqttSender := transforms.NewMQTTSecretSender(config, mqttConfig.GetPersistOnError(service))
		mlModelReadySubs.MqttSender = mqttSender
	}
	return mlModelReadySubs
}

// Infers the corresponding status based of success/failure and reports it back to ML Management at core
func (agentPublisher *MLAgentMQTTPublisher) BuildModelDownloadStatusPayload(
	ctx interfaces.AppFunctionContext,
	modelDeployCommand ml_model.ModelDeployCommand,
	isSuccess bool,
	message string,
) ml_model.ModelDeploymentStatus {

	var modelCommandStatus ml_model.ModelDeployStatusCode
	if isSuccess {
		switch strings.ToLower(modelDeployCommand.CommandName) {
		case "deploy":
			modelCommandStatus = ml_model.ModelDeployed
		case "undeploy":
			modelCommandStatus = ml_model.ModelUnDeployed
		}
		if modelDeployCommand.CommandName == "SyncMLEventConfig" {
			modelCommandStatus = ml_model.ModelEventConfigSyncSuccess
		}
		ctx.LoggingClient().Infof("MLCommand status: %s", modelCommandStatus)
	} else {
		//Set the failure status's since it is dependent on the command that was fired
		switch strings.ToLower(modelDeployCommand.CommandName) {
		case "deploy":
			modelCommandStatus = ml_model.ModelDeploymentFailed
		case "undeploy":
			modelCommandStatus = ml_model.ModelUnDeploymentFailed
		}
		if modelDeployCommand.CommandName == "SyncMLEventConfig" {
			modelCommandStatus = ml_model.ModelEventConfigSyncFailed
		}
		ctx.LoggingClient().Errorf("MLCommand Status: %s, Error: %v", modelCommandStatus.String(), message)
	}
	modelEdgeSyncStatus := ml_model.ModelDeploymentStatus{
		MLAlgorithm:       modelDeployCommand.MLAlgorithm,
		MLModelConfigName: modelDeployCommand.MLModelConfigName,
		ModelVersion:      modelDeployCommand.ModelVersion,

		DeploymentStatusCode:    modelCommandStatus,
		DeploymentStatus:        modelCommandStatus.String(),
		ModelDeploymentTimeSecs: time.Now().Unix(),
		Message:                 message,
	}
	if len(modelDeployCommand.TargetNodes) > 0 {
		modelEdgeSyncStatus.NodeId = modelDeployCommand.TargetNodes[0]
	} else {
		//God knows how we come here, but saw an exception so the handling
		ctx.LoggingClient().Errorf("target node nil while publishing deployment status ")
	}

	modelSyncBytes, _ := util.CoerceType(modelEdgeSyncStatus)
	ctx.SetResponseData(modelSyncBytes)

	return modelEdgeSyncStatus
}
