package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	redisdb "hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
)

var (
	u                      *utils.HedgeMockUtils
	testMlAlgorithmName    = "TestAlgo"
	testMlModelConfigName  = "TestMlModelConfig"
	testConnectionConfig   *config.MLMgmtConfig
	testDataSourceProvider service.DataStoreProvider
	testDbConfig           *redisdb.DatabaseConfig
	testError              = errors.New("dummy error")
	testHedgeError         = hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"hedge dummy error",
	)
	testImageDigest = "sha256:dummydigest"
	timestamp       = time.Now().Unix()
	testUserId      = "admin"
)

var jobScriptMock = `
metadata:
  name: hedge-auto-job
  namespace: __NAMESPACE__
spec:
  template:
    spec:
      containers:
        - name: hedge-auto-job
          image: __DOCKER_REGISTRY_PREFIX__/lpade:hedge-ml-trg-anomaly-autoencoder
`

var templateMock = `
{
  "name": "AutoEncoder",
  "type": "Anomaly",
  "jobScript": "",
  "description": "",
  "algorithmSchema": {
    "template": {
      "config": {
        "properties": {
          "name": {
            "type": "string",
            "const": "AutoEncoder"
          }
        }
      }
    }
  }
}
`

func init() {
	u = utils.NewApplicationServiceMock(
		map[string]string{
			"DataStore_Provider": "ADE",
			"MetaDataServiceUrl": "http://localhost:59881",
			"RedisHost":          "edgex-redis",
			"RedisPort":          "6349",
			"RedisName":          "redisdb",
			"RedisUsername":      "default",
			"RedisPassword":      "bn784738bnxpassword",
			"ADE_TENANT_URL":     "http://dummy-url.com",
			"ADE_TENANT_ID":      "dummy-id",
		},
	)
	testDbConfig = redisdb.NewDatabaseConfig()
	testDbConfig.LoadAppConfigurations(u.AppService)
	testConnectionConfig = config.NewMLMgmtConfig()
	_ = testConnectionConfig.LoadCoreAppConfigurations(u.AppService)
}

func buildTestMlAlgoDefinition(
	mlAlgoDefinitionName string,
	mlAlgoDefinitionType string,
	predictionPort int64,
	isOotb bool,
) *config.MLAlgorithmDefinition {
	return &config.MLAlgorithmDefinition{
		Name:                         mlAlgoDefinitionName,
		Description:                  "Test description",
		Type:                         mlAlgoDefinitionType,
		Enabled:                      true,
		DefaultPredictionEndpointURL: "/api/v3/predict",
		DefaultPredictionPort:        predictionPort,
		OutputFeaturesPredefined:     true,
		AllowDeploymentAtEdge:        true,
		PredictionImagePath:          "test-prediction-image-path:test-tag",
		PredictionImageDigest:        testImageDigest,
		TrainerImagePath:             "lpade:test-training-image-path",
		TrainerImageDigest:           testImageDigest,
		IsOotb:                       isOotb,
	}
}

func buildTestMLModelConfigByMLModelDataConfig(
	mlModelDataConfig config.MLModelDataConfig,
) *config.MLModelConfig {
	return &config.MLModelConfig{
		Name:                 testMlModelConfigName,
		Version:              "1",
		Description:          "Test description",
		MLAlgorithm:          testMlAlgorithmName,
		MLDataSourceConfig:   mlModelDataConfig,
		Message:              "Test message with {{.deviceName}} placeholder",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgo/TestProfile",
		TrainedModelCount:    0,
		MLAlgorithmType:      "TestType",
	}
}

func buildTestMLModelConfig(
	algoType string,
	featuresByProfile map[string][]config.Feature,
	featureIndexMap map[string]int,
	groupOrJoinKeysValid []string,
) *config.MLModelConfig {
	return &config.MLModelConfig{
		MLAlgorithmType: algoType,
		MLDataSourceConfig: config.MLModelDataConfig{
			FeaturesByProfile:        featuresByProfile,
			FeatureNameToColumnIndex: featureIndexMap,
			GroupOrJoinKeys:          groupOrJoinKeysValid,
			InputContextCount:        1,
			TrainingDataSourceConfig: config.TrainingDataSourceConfig{
				SamplingIntervalSecs:           61,
				DataCollectionTotalDurationSec: 6100,
			},
			PredictionDataSourceConfig: config.PredictionDataSourceConfig{
				SamplingIntervalSecs: 60,
			},
		},
	}
}

func buildTestMLModelDataConfig(
	features map[string][]config.Feature,
	predictionEndPointURL string,
	topicName string,
) config.MLModelDataConfig {
	return config.MLModelDataConfig{
		PredictionDataSourceConfig: config.PredictionDataSourceConfig{
			PredictionEndPointURL: predictionEndPointURL,
			TopicName:             topicName,
		},
		FeaturesByProfile: features,
	}
}

func buildTestModelDeployments(
	deploymentStatusCode ml_model.ModelDeployStatusCode,
	deploymentStatus string,
) []ml_model.ModelDeploymentStatus {
	return []ml_model.ModelDeploymentStatus{
		{
			MLAlgorithm:             testMlAlgorithmName,
			MLModelConfigName:       testMlModelConfigName,
			ModelName:               "DecisionTree_Config2_2",
			NodeId:                  "Node-1",
			NodeDisplayName:         "Node1",
			DeploymentStatusCode:    deploymentStatusCode,
			DeploymentStatus:        deploymentStatus,
			ModelVersion:            2,
			ModelDeploymentTimeSecs: timestamp,
			IsModelDeprecated:       true,
			PermittedOption:         "none",
			Message:                 "",
		},
	}
}

func buildHTTPResponse(method string) *http.Response {
	testHTTPResponseData := map[string]interface{}{
		"response": []interface{}{
			map[string]interface{}{
				"name":       testMlAlgorithmName,
				"id":         "12345",
				"templateId": "67890",
			},
		},
	}
	bodyBytes, _ := json.Marshal(testHTTPResponseData)

	switch method {
	case http.MethodGet:
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}
	case http.MethodPost:
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}
	case http.MethodPut:
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}
	case http.MethodDelete:
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	}
	return nil
}

func readFileMock(path string) ([]byte, error) {
	if path == "res/ade/jobScript.yml" {
		return []byte(jobScriptMock), nil
	} else if path == "res/ade/hedge_algo_template.json" {
		return []byte(templateMock), nil
	}
	return nil, errTest
}

// FaultyReader is a mock reader that always fails
type FaultyReader struct{}

func (FaultyReader) Read(p []byte) (n int, err error) {
	return 0, errTest
}
