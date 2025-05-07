/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	hedgeErrors "hedge/common/errors"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	"hedge/common/db"
	redis2 "hedge/common/db/redis"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"github.com/gomodule/redigo/redis"
)

// db.MLTrainedModel|trgConfig|modelVersion -> modelKey
// db.MLTrainedModel -> All models
// db.MLTrainedModel|trgConfig -> All models by Config
// db.MLTrainedModel|trgConfig|ver -> All versions for a trainingConfig
func (dbClient *DBClient) SaveMLModel(mlModel ml_model.MLModel) hedgeErrors.HedgeError {

	lc := dbClient.client.Logger

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error saving ML model %s", mlModel.ModelName)

	modelDbKey := buildModelConfigKey(mlModel.MLAlgorithm, mlModel.MLModelConfigName, mlModel.ModelVersion)
	modelKeyByConfig := buildModelBaseKey(mlModel.MLAlgorithm, mlModel.MLModelConfigName)

	// Need to enable lookup by: algo+config name, modelName and config->modelVersions

	m, err := marshalObject(mlModel)
	if err != nil {
		lc.Errorf("Error marshalling model: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", modelDbKey, m)
	// All Models
	_ = conn.Send("SADD", db.MLTrainedModel, modelDbKey)
	// By Training Config
	_ = conn.Send("SADD", modelKeyByConfig, modelDbKey)
	// By ModelName
	//_ = conn.Send("HSET", db.MLModel+":name", key, key)
	// All model versions for a given trainingConfig
	_ = conn.Send("SADD", modelKeyByConfig+"|"+"ver", mlModel.ModelVersion)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error while saving ML training job in db: %v", err)
	} else {
		//Mark other versions as deprecated
		if mlModel.ModelVersion > 1 {
			lc.Info("Will now deprecate the old versions")
			// Check for old versions
			for i := 1; i < int(mlModel.ModelVersion); i++ {
				modelDbKey = buildModelConfigKey(mlModel.MLAlgorithm, mlModel.MLModelConfigName, int64(i))
				oldModel, err := redis.Bytes(conn.Do("GET", modelDbKey))
				if err != nil {
					continue
				}
				var model ml_model.MLModel
				err = json.Unmarshal(oldModel, &model)
				if err == nil && !model.IsModelDeprecated {
					lc.Info("Deprecating the old model version")
					model.IsModelDeprecated = true
					m, _ := marshalObject(model)
					_, _ = conn.Do("SET", modelDbKey, m)
				}
			}
		}
		// Ignore error in this step
		return nil
	}

	return nil
}

// if mlAlgo or trainingConfig is empty, get all models across all configs
func (dbClient *DBClient) GetModels(mlAlgorithm string, trainingConfigName string) ([]ml_model.MLModel, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error getting models for algorithm '%s'", mlAlgorithm)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	key := db.MLTrainedModel
	if trainingConfigName != "" && mlAlgorithm != "" {
		key = buildModelBaseKey(mlAlgorithm, trainingConfigName)
	}

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		lc.Errorf("Error while getting models by training config: %v", err)
		return make([]ml_model.MLModel, 0), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]ml_model.MLModel, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			lc.Errorf("%s: Error while unmarshaling object: %v", errorMessage, err)
			return make([]ml_model.MLModel, 0), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return d, nil
}

func (dbClient *DBClient) GetLatestModelVersion(mlAlgorithm string, trainingConfigName string) (int64, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error getting model version for algorithm %s", mlAlgorithm)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	baseModelKey := buildModelBaseKey(mlAlgorithm, trainingConfigName)

	ids, err := redis.ByteSlices(conn.Do("SMEMBERS", baseModelKey+"|"+"ver"))

	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		if errors.Is(err, redis.ErrNil) {
			return 0, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage)
		}

		return 0, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if len(ids) == 0 {
		return 0, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage)
	}

	// find the largest id
	var modelVesion int64 = 1
	for _, ver := range ids {
		var version int64
		err := json.Unmarshal(ver, &version)
		if err != nil {
			lc.Errorf("Unmarshal Error: %v", err)
			continue
		}
		if version > modelVesion {
			modelVesion = version
		}
	}
	return modelVesion, nil
}

func (dbClient *DBClient) GetLatestModelsByConfig(mlAlgorithm string, trainingConfigName string) ([]ml_model.MLModel, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	models, err := dbClient.GetModels(mlAlgorithm, trainingConfigName)
	if err != nil || len(models) == 0 {
		return nil, err
	}
	latestModelMap := make(map[string]ml_model.MLModel, 0)
	//latestModel := models[0]
	for _, model := range models {
		lc.Infof("model config: %s", model.MLModelConfigName)
		if _, found := latestModelMap[model.MLModelConfigName]; !found {
			latestModelMap[model.MLModelConfigName] = model
		} else if latestModelMap[model.MLModelConfigName].ModelVersion < model.ModelVersion {
			latestModelMap[model.MLModelConfigName] = model
		}
	}

	// Use maps.Values(latestModelMap) from goland 1.18 onwards
	latestModels := make([]ml_model.MLModel, len(latestModelMap))
	i := 0
	for _, model := range latestModelMap {
		latestModels[i] = model
		i++
	}
	return latestModels, nil
}

func (dbClient *DBClient) DeleteMLModelsByModelVersion(mlAlgorithm string, trainingConfigName string, modelVersion int64) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error deleting models for algorithm %s", mlAlgorithm)

	models, err := dbClient.GetModels(mlAlgorithm, trainingConfigName)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return err
	}
	for _, model := range models {
		if modelVersion > 0 && modelVersion == model.ModelVersion {
			err = deleteMLModel(conn, lc, mlAlgorithm, trainingConfigName, model.ModelVersion)
			if err != nil {
				lc.Errorf("%s: %v", errorMessage, err)
				return err
			}
		}
	}

	return nil
}

func (dbClient *DBClient) DeleteMLModelsByConfig(mlAlgorithm string, trainingConfigName string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error deleting models for algorithm %s", mlAlgorithm)

	models, err := dbClient.GetModels(mlAlgorithm, trainingConfigName)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return err
	}
	for _, model := range models {
		err = deleteMLModel(conn, lc, mlAlgorithm, trainingConfigName, model.ModelVersion)
		if err != nil {
			lc.Errorf("%s: %v", errorMessage, err)
			return err
		}
	}
	return nil
}

// To be fixed
func deleteMLModel(conn redis.Conn, lc logger.LoggingClient, mlAlgorithm string, trainingConfigName string, modelVersion int64) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting model version for algorithm %s", mlAlgorithm)

	modelDbKey := buildModelConfigKey(mlAlgorithm, trainingConfigName, modelVersion)
	modelKeyByConfig := buildModelBaseKey(mlAlgorithm, trainingConfigName)
	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", modelDbKey)
	_ = conn.Send("SREM", db.MLTrainedModel, modelDbKey)
	_ = conn.Send("SREM", modelKeyByConfig, modelDbKey)
	_ = conn.Send("SREM", modelKeyByConfig+"|"+"ver", modelVersion)
	_, err := conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error While Deleting Anomaly Training Job in DB: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func buildModelBaseKey(mlAlgorithm string, trainingConfigName string) string {
	return db.MLTrainedModel + "|" + mlAlgorithm + "|" + trainingConfigName
}
func buildModelConfigKey(mlAlgorithm string, trainingConfigName string, modelVersion int64) string {
	return buildModelBaseKey(mlAlgorithm, trainingConfigName) + "|" + fmt.Sprintf("%d", modelVersion)
}
func (dbClient *DBClient) modelKey(name string) string {
	return db.MLTrainedModel + "|" + name
}
