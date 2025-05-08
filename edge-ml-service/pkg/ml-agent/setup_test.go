package ml_agent_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/stretchr/testify/mock"
)

var (
	u                                        *utils.HedgeMockUtils
	testModelDeploymentStatusSuccess         ml_model.ModelDeploymentStatus
	testModelUnDeploymentStatusSuccess       ml_model.ModelDeploymentStatus
	testModelDeploymentStatusFailure         ml_model.ModelDeploymentStatus
	testModelDeploymentStatusUndeployFailure ml_model.ModelDeploymentStatus
	errDummy                                 = errors.New("dummy error")
	testHedgeError                           = hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"hedge dummy error",
	)
	testMlAlgorithmName   = "TestAlgo"
	testMlModelConfigName = "TestConfig"
	testMlModelDir        = "testMlModelDir"
	timestamp             = time.Now().Unix()
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{
		"ModelDeployTopic":           "deploy-topic",
		"LocalMLModelDir":            testMlModelDir,
		"ModelDownloadTopic":         "download-topic",
		"ReinitializeEndpoint":       "/reinitialize",
		"ModelDownloadEndpoint":      "/download",
		"ModelDownloadEnabled":       "true",
		"MqttUserName":               "mqtt-user",
		"MqttPassword":               "mqtt-password",
		"MqttScheme":                 "tcp",
		"MqttServer":                 "localhost",
		"MqttPort":                   "1883",
		"InsecureSkipVerify":         "false",
		"ImageRegistry":              "test-registry",
		"ExposePredictContainerPort": "true",
	})
	u.AppFunctionContext.On("SetResponseData", mock.Anything).Return()
	testModelDeploymentStatusSuccess = buildModelDeploymentStatus(
		ml_model.ModelDeployed, 4, "Model deployed successfully",
	)
	testModelUnDeploymentStatusSuccess = buildModelDeploymentStatus(
		ml_model.ModelUnDeployed, 3, "Model undeployed successfully",
	)
	testModelDeploymentStatusFailure = buildModelDeploymentStatus(
		ml_model.ModelDeploymentFailed, 7, "Deployment of model failed",
	)
	testModelDeploymentStatusUndeployFailure = buildModelDeploymentStatus(
		ml_model.ModelUnDeploymentFailed, 6, "Undeployment of model failed",
	)
}

func buildTestMLEventConfigs() []config.MLEventConfig {
	conditions := []config.Condition{
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.BETWEEN_OPERATOR,
					LowerThreshold: 2,
					UpperThreshold: 3,
				},
			},
			SeverityLevel: "MINOR",
		},
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.BETWEEN_OPERATOR,
					LowerThreshold: 3,
					UpperThreshold: 4,
				},
			},
			SeverityLevel: "MAJOR",
		},
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.GREATER_THAN_OPERATOR,
					ThresholdValue: 4,
				},
			},
			SeverityLevel: "CRITICAL",
		},
	}

	mlEventConfig := config.MLEventConfig{
		MLAlgorithm:       testMlAlgorithmName,
		MlModelConfigName: testMlModelConfigName,
		EventName:         testMlModelConfigName + "Event",
		Description:       "Anomaly detected",
		Conditions:        conditions, StabilizationPeriodByCount: 3,
	}

	return []config.MLEventConfig{mlEventConfig}
}

func buildTestModelDeployCommand(
	commandName, algorithmName, configName string,
	version int64,
	nodes []string,
) ml_model.ModelDeployCommand {
	return ml_model.ModelDeployCommand{
		CommandName:       commandName,
		MLAlgorithm:       algorithmName,
		MLModelConfigName: configName,
		ModelVersion:      version,
		TargetNodes:       nodes,
		Algorithm: &config.MLAlgorithmDefinition{
			PredictionImagePath: "test-predictor:latest",
		},
	}
}

func buildModelDeploymentStatus(
	deploymentStatus ml_model.ModelDeployStatusCode,
	deploymentStatusCode ml_model.ModelDeployStatusCode,
	message string,
) ml_model.ModelDeploymentStatus {
	return ml_model.ModelDeploymentStatus{
		MLAlgorithm:             testMlAlgorithmName,
		MLModelConfigName:       testMlModelConfigName,
		ModelName:               "ExampleModel",
		NodeId:                  "Node1",
		NodeDisplayName:         "Node 1",
		DeploymentStatusCode:    deploymentStatusCode,
		DeploymentStatus:        deploymentStatus.String(),
		ModelVersion:            1,
		ModelDeploymentTimeSecs: timestamp,
		IsModelDeprecated:       false,
		PermittedOption:         "deploy",
		Message:                 message,
	}
}

func updateTimestampIfModelDeploymentStatus(data interface{}) {
	if data != nil {
		if result, ok := data.(*ml_model.ModelDeploymentStatus); ok {
			result.ModelDeploymentTimeSecs = timestamp
		}
	}
}

func createDummyZip(t *testing.T, testDir string) *bytes.Buffer {
	t.Helper()

	err := os.MkdirAll(testDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	_, err = zipWriter.Create("dummy.txt")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	zipFilePath := filepath.Join(testDir, "dummy.zip")
	err = os.WriteFile(zipFilePath, buf.Bytes(), 0644)
	if err != nil {
		t.Fatalf("Failed to write zip file: %v", err)
	}

	return buf
}

func setupTestDirWithConfig(t *testing.T, testDir string) {
	t.Helper()

	// Create test directory for the ML model
	fullPath := filepath.Join(
		testDir,
		testMlAlgorithmName,
		testMlModelConfigName,
		"hedge_export",
		"assets",
	)
	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	configFilePath := filepath.Join(fullPath, "config.json")
	configContent := `{
		"mlModelConfig": {
			"name": "TestConfig"
		},
		"mlAlgoDefinition": {
			"name": "TestAlgo"
		}
	}`

	err = os.WriteFile(configFilePath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}
}
