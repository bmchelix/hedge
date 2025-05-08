/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"hedge/common/db"
	"hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"github.com/go-redsync/redsync/v4"
)

type DBClient struct {
	client *redis.DBClient
}

var MLDbClientImpl MLDbInterface

type MLDbInterface interface {
	redis.CommonRedisDBInterface
	GetDbClient(dbConfig *db.DatabaseConfig) MLDbInterface
	// BYOM changes
	GetAlgorithm(algoName string) (*config.MLAlgorithmDefinition, hedgeErrors.HedgeError)
	GetAllAlgorithms() ([]*config.MLAlgorithmDefinition, hedgeErrors.HedgeError)
	CreateAlgorithm(algo config.MLAlgorithmDefinition) hedgeErrors.HedgeError
	UpdateAlgorithm(algo config.MLAlgorithmDefinition) hedgeErrors.HedgeError
	DeleteAlgorithm(algoName string) hedgeErrors.HedgeError
	AddMLEventConfig(eventConfig config.MLEventConfig) (config.MLEventConfig, hedgeErrors.HedgeError)
	UpdateMLEventConfig(existingMLEvent config.MLEventConfig, eventConfig config.MLEventConfig) (config.MLEventConfig, hedgeErrors.HedgeError)
	GetMLEventConfigByName(mlAlgorithmName string, mlModelConfigName string, mlEventName string) (config.MLEventConfig, hedgeErrors.HedgeError)
	GetAllMLEventConfigsByConfig(mlAlgorithmName, mlModelConfigName string) ([]config.MLEventConfig, hedgeErrors.HedgeError)
	DeleteMLEventConfigByName(mlAlgorithmName string, mlModelConfigName string, eventName string) hedgeErrors.HedgeError
	SaveMLModelConfig(mlModelConfig config.MLModelConfig) (string, hedgeErrors.HedgeError)
	GetMlModelConfig(mlAlgorithm string, mlModelConfigName string) (config.MLModelConfig, hedgeErrors.HedgeError)
	GetAllMLModelConfigs(mlAlgorithm string) ([]config.MLModelConfig, hedgeErrors.HedgeError)
	DeleteMLModelConfig(mlAlgorithm string, mlModelConfigName string) hedgeErrors.HedgeError
	UpdateModelDeployment(modelDeployment ml_model.ModelDeploymentStatus) hedgeErrors.HedgeError
	GetDeploymentsByConfig(mlAlgorithm string, mlModelConfigName string) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError)
	GetDeploymentsByNode(nodeName string) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError)
	GetDeploymentsByModelVersion(mlAlgorithmName string, mlModelConfigName string, modelVersion int64) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError)
	AddMLTrainingJob(trainingJob job.TrainingJobDetails) (string, hedgeErrors.HedgeError)
	UpdateMLTrainingJob(trainingJob job.TrainingJobDetails) (string, hedgeErrors.HedgeError)
	GetMLTrainingJob(id string) (job.TrainingJobDetails, hedgeErrors.HedgeError)
	GetMLTrainingJobsByConfig(mlModelConfigName string, jobStatus string) ([]job.TrainingJobDetails, hedgeErrors.HedgeError)
	MarkOldTrainingDataDeprecated(mlModelConfigName string, currentJobName string) hedgeErrors.HedgeError
	GetMLTrainingJobs(key string) ([]job.TrainingJobDetails, hedgeErrors.HedgeError)
	DeleteMLTrainingJobs(jobName string) hedgeErrors.HedgeError
	SaveMLModel(mlModel ml_model.MLModel) hedgeErrors.HedgeError
	GetModels(mlAlgorithm string, mlModelConfigName string) ([]ml_model.MLModel, hedgeErrors.HedgeError)
	GetLatestModelVersion(mlAlgorithm string, mlModelConfigName string) (int64, hedgeErrors.HedgeError)
	GetLatestModelsByConfig(mlAlgorithm string, mlModelConfigName string) ([]ml_model.MLModel, hedgeErrors.HedgeError)
	DeleteMLModelsByModelVersion(mlAlgorithm string, mlModelConfigName string, modelVersion int64) hedgeErrors.HedgeError
	DeleteMLModelsByConfig(mlAlgorithm string, mlModelConfigName string) hedgeErrors.HedgeError

	/*	GetModel(modelName string) (*config.MLModelConfig, hedgeErrors.HedgeError)
		CreateModel(model config.MLModelConfig) hedgeErrors.HedgeError
		UpdateModel(model config.MLModelConfig) hedgeErrors.HedgeError
		DeleteModel(modelName string) hedgeErrors.HedgeError*/
}

func init() {
	MLDbClientImpl = &DBClient{}
}

func NewDBClient(dbConfig *db.DatabaseConfig) MLDbInterface {
	return MLDbClientImpl.GetDbClient(dbConfig)
}

func (dbClient *DBClient) GetDbClient(dbConfig *db.DatabaseConfig) MLDbInterface {
	dbc := redis.CreateDBClient(dbConfig)
	dbc.Logger = newLoggingClient()
	return &DBClient{client: dbc}
}

func (dbClient *DBClient) IncrMetricCounterBy(key string, value int64) (int64, hedgeErrors.HedgeError) {
	return dbClient.client.IncrMetricCounterBy(key, value)
}

func (dbClient *DBClient) SetMetricCounter(key string, value int64) hedgeErrors.HedgeError {
	return dbClient.client.SetMetricCounter(key, value)
}

func (dbClient *DBClient) GetMetricCounter(key string) (int64, hedgeErrors.HedgeError) {
	return dbClient.client.GetMetricCounter(key)
}

func (dbClient *DBClient) AcquireRedisLock(name string) (*redsync.Mutex, hedgeErrors.HedgeError) {
	return dbClient.client.AcquireRedisLock(name)
}
