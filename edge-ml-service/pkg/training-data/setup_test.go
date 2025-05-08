package training_data_test

import (
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
)

const TRAINING_DATA_FILE string = "testFile.csv"

var u *utils.HedgeMockUtils
var mockmlModelConfig *config.MLModelConfig
var appConfig *config.MLMgmtConfig

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{"DataStore_Provider": "ADE", "MetaDataServiceUrl": "http://localhost:59881", "RedisHost": "edgex-redis",
		"RedisPort": "6349", "RedisName": "redisdb", "RedisUsername": "default", "RedisPassword": "bn784738bnxpassword",
		"ADE_TENANT_URL": "http://dummy-url.com", "ADE_TENANT_ID": "dummy-id"})

	mockmlModelConfig = &config.MLModelConfig{
		Name:                 TRAINING_DATA_FILE,
		Version:              "1.0",
		Description:          "Test description",
		MLAlgorithm:          "TestAlgorithm",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		TrainedModelCount:    0,
		MLDataSourceConfig: config.MLModelDataConfig{
			TrainingDataSourceConfig: config.TrainingDataSourceConfig{
				DataCollectionTotalDurationSec: 5,
				SamplingIntervalSecs:           2,
			},
		},
	}
}
