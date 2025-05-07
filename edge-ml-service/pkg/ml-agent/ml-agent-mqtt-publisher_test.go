/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package ml_agent_test

import (
	"hedge/edge-ml-service/pkg/dto/ml_model"
	mlagent "hedge/edge-ml-service/pkg/ml-agent"
	mlagentmocks "hedge/mocks/hedge/edge-ml-service/pkg/ml-agent"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMLAgentMQTTPublisher_NewMLModelReadySubscriber(t *testing.T) {
	t.Run("NewMLModelReadySubscriber - Passed", func(t *testing.T) {
		subscriber := mlagent.NewMLModelReadySubscriber(u.AppService, &mlagent.MLEdgeAgentConfig{})

		assert.NotNil(t, subscriber, "Expected non-nil subscriber")
		assert.IsType(t, &mlagent.MLAgentMQTTPublisher{}, subscriber, "Expected subscriber of type MLAgentMQTTPublisher")
	})
}

func TestMLAgentMQTTPublisher_GetMLAgentMQTTPublisher(t *testing.T) {
	t.Run("GetMLAgentMQTTPublisher - Passed", func(t *testing.T) {
		mockedMlAgentConfig := mlagentmocks.MockMLEdgeAgentConfigInterface{}
		mockedMlAgentConfig.On("GetModelDownloadTopic").Return("testDownloadTopic")

		agentPublisher := &mlagent.MLAgentMQTTPublisher{}
		got := agentPublisher.GetMLAgentMQTTPublisher(u.AppService, &mockedMlAgentConfig)

		assert.NotNil(t, got, "Expected non-nil MLAgentMQTTPublisher")
		assert.NotNil(t, got.(*mlagent.MLAgentMQTTPublisher).MqttSender, "Expected non-nil MqttSender")
	})
}

func TestMLAgentMQTTPublisher_BuildModelDownloadStatusPayload(t *testing.T) {
	t.Run("BuildModelDownloadStatusPayload - Passed (Deploy)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand("Deploy", testMlAlgorithmName, testMlModelConfigName, 1, []string{"Node1"})
		expected := ml_model.ModelDeploymentStatus{
			MLAlgorithm:             testMlAlgorithmName,
			MLModelConfigName:       testMlModelConfigName,
			ModelVersion:            1,
			DeploymentStatusCode:    ml_model.ModelDeployed,
			DeploymentStatus:        "ModelDeployed",
			ModelDeploymentTimeSecs: timestamp,
			Message:                 "Deployment successful",
			NodeId:                  "Node1",
		}

		mockedMlAgentConfig := &mlagentmocks.MockMLEdgeAgentConfigInterface{}

		agentPublisher := &mlagent.MLAgentMQTTPublisher{
			AppConfig: mockedMlAgentConfig,
		}

		got := agentPublisher.BuildModelDownloadStatusPayload(u.AppFunctionContext, testModelDeployCommand, true, "Deployment successful")
		got.ModelDeploymentTimeSecs = timestamp
		assert.Equal(t, expected, got)
	})
	t.Run("BuildModelDownloadStatusPayload - Passed (UnDeploy)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand("UnDeploy", testMlAlgorithmName, testMlModelConfigName, 1, []string{"Node1"})
		expected := ml_model.ModelDeploymentStatus{
			MLAlgorithm:             testMlAlgorithmName,
			MLModelConfigName:       testMlModelConfigName,
			ModelVersion:            1,
			DeploymentStatusCode:    ml_model.ModelUnDeployed,
			DeploymentStatus:        "ModelUnDeployed",
			ModelDeploymentTimeSecs: timestamp,
			Message:                 "UnDeployment successful",
			NodeId:                  "Node1",
		}

		mockedMlAgentConfig := &mlagentmocks.MockMLEdgeAgentConfigInterface{}

		agentPublisher := &mlagent.MLAgentMQTTPublisher{
			AppConfig: mockedMlAgentConfig,
		}

		got := agentPublisher.BuildModelDownloadStatusPayload(u.AppFunctionContext, testModelDeployCommand, true, "UnDeployment successful")
		got.ModelDeploymentTimeSecs = timestamp
		assert.Equal(t, expected, got)
	})
	t.Run("BuildModelDownloadStatusPayload - Failed (Undeployment failed)", func(t *testing.T) {
		testModelUnDeployCommand := buildTestModelDeployCommand("UnDeploy", testMlAlgorithmName, testMlModelConfigName, 2, []string{"Node3", "Node4"})
		expected := ml_model.ModelDeploymentStatus{
			MLAlgorithm:             testMlAlgorithmName,
			MLModelConfigName:       testMlModelConfigName,
			ModelVersion:            2,
			DeploymentStatusCode:    ml_model.ModelUnDeploymentFailed,
			DeploymentStatus:        "ModelUnDeploymentFailed",
			ModelDeploymentTimeSecs: timestamp,
			Message:                 "Undeployment failed",
			NodeId:                  "Node3",
		}

		mockedMlAgentConfig := &mlagentmocks.MockMLEdgeAgentConfigInterface{}

		agentPublisher := &mlagent.MLAgentMQTTPublisher{
			EdgexSdk:  u.AppService,
			AppConfig: mockedMlAgentConfig,
		}

		got := agentPublisher.BuildModelDownloadStatusPayload(u.AppFunctionContext, testModelUnDeployCommand, false, "Undeployment failed")
		got.ModelDeploymentTimeSecs = timestamp
		assert.Equal(t, expected, got)
	})
	t.Run("BuildModelDownloadStatusPayload - Failed (failed for SyncMLEventConfig cmd)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand("SyncMLEventConfig", testMlAlgorithmName, testMlModelConfigName, 2, []string{"Node3", "Node4"})
		expected := ml_model.ModelDeploymentStatus{
			MLAlgorithm:             testMlAlgorithmName,
			MLModelConfigName:       testMlModelConfigName,
			ModelVersion:            2,
			DeploymentStatusCode:    ml_model.ModelEventConfigSyncSuccess,
			DeploymentStatus:        "ModelEventConfigSyncSuccess",
			ModelDeploymentTimeSecs: timestamp,
			Message:                 "Undeployment failed",
			NodeId:                  "Node3",
		}

		mockedMlAgentConfig := &mlagentmocks.MockMLEdgeAgentConfigInterface{}

		agentPublisher := &mlagent.MLAgentMQTTPublisher{
			EdgexSdk:  u.AppService,
			AppConfig: mockedMlAgentConfig,
		}

		got := agentPublisher.BuildModelDownloadStatusPayload(u.AppFunctionContext, testModelDeployCommand, true, "Undeployment failed")
		got.ModelDeploymentTimeSecs = timestamp
		assert.Equal(t, expected, got)
	})
}
