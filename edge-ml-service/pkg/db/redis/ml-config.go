/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	hedgeErrors "hedge/common/errors"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"github.com/gomodule/redigo/redis"
)

func (dbClient *DBClient) AddMLEventConfig(eventConfig config.MLEventConfig) (config.MLEventConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error adding event config %v", eventConfig.EventName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	modelDbKey := buildEventConfigKey(
		eventConfig.MLAlgorithm,
		eventConfig.MlModelConfigName,
		eventConfig.EventName,
	)

	// Check if key already exists in db
	exists, err := redis.Bool(conn.Do("HEXISTS", db2.MLEventConfig+":name", modelDbKey))
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if exists {
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: %s", errorMessage, "Already exists"))
	}

	// Add to db
	err = addMLEventConfig(conn, eventConfig, modelDbKey)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if conError := conn.Flush(); conError != nil {
		lc.Errorf("%s: failed to execute commands %v", errorMessage, conError)
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	// Retrieve the created config to verify it was stored correctly
	createdMLEventConfig, hErr := dbClient.GetMLEventConfigByName(eventConfig.MLAlgorithm, eventConfig.MlModelConfigName, eventConfig.EventName)
	if hErr != nil {
		lc.Errorf("Failed to save ML Event Config in DB: %s", eventConfig.EventName)
		return config.MLEventConfig{}, hErr
	}

	lc.Infof("Successfully saved and verified ML Event Config in DB: %s", createdMLEventConfig.EventName)
	return createdMLEventConfig, nil
}

func (dbClient *DBClient) UpdateMLEventConfig(existingEventConfig config.MLEventConfig, eventConfig config.MLEventConfig) (config.MLEventConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error updating event config %v", eventConfig.EventName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	_ = conn.Send("MULTI")

	// Step 1: Delete the existing config
	err := deleteMLEventConfigByName(conn, lc, existingEventConfig.MLAlgorithm, existingEventConfig.MlModelConfigName, existingEventConfig.EventName)
	if err != nil {
		_, _ = conn.Do("DISCARD") // Discard transaction on error
		lc.Errorf("%s: %v", errorMessage, err)
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	// Step 2: Add the new config
	modelDbKey := buildEventConfigKey(eventConfig.MLAlgorithm, eventConfig.MlModelConfigName, eventConfig.EventName)
	addErr := addMLEventConfig(conn, eventConfig, modelDbKey)
	if addErr != nil {
		_, _ = conn.Do("DISCARD") // Discard transaction on error
		// Attempt to restore the original config
		modelDbKey = buildEventConfigKey(existingEventConfig.MLAlgorithm, existingEventConfig.MlModelConfigName, existingEventConfig.EventName)
		if restoreErr := addMLEventConfig(conn, existingEventConfig, modelDbKey); restoreErr != nil {
			lc.Errorf("%s: failed to restore existing config after rollback: %v", errorMessage, restoreErr)
			return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}

		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	// Step 3: Commit transaction
	_, connErr := conn.Do("EXEC")
	if connErr != nil {
		// If the transaction fails, attempt to restore the original ML event config
		modelDbKey = buildEventConfigKey(existingEventConfig.MLAlgorithm, existingEventConfig.MlModelConfigName, existingEventConfig.EventName)
		if restoreErr := addMLEventConfig(conn, existingEventConfig, modelDbKey); restoreErr != nil {
			lc.Errorf("%s: failed to restore existing config after rollback: %v", errorMessage, restoreErr)
			return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}

		lc.Errorf("failed to update ML event config %s: transaction failed: %v", eventConfig.EventName, connErr)
		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	// Step 4: Return the newly created configuration
	updatedConfig, err := dbClient.GetMLEventConfigByName(eventConfig.MLAlgorithm, eventConfig.MlModelConfigName, eventConfig.EventName)
	if err != nil {
		lc.Errorf("Failed to update ML Event Config in DB: %s", eventConfig.EventName)
		return config.MLEventConfig{}, err
	}
	lc.Infof("Successfully saved and verified ML Event Config in DB: %s", updatedConfig.EventName)
	return updatedConfig, nil
}

func addMLEventConfig(conn redis.Conn, eventConfig config.MLEventConfig, modelDbKey string) error {
	m, err := marshalObject(&eventConfig)
	if err != nil {
		return err
	}

	if err := conn.Send("SET", modelDbKey, m); err != nil {
		return fmt.Errorf("failed to queue SET: %v", err)
	}
	if err := conn.Send("SADD", db2.MLEventConfig, modelDbKey); err != nil {
		return fmt.Errorf("failed to queue SADD: %v", err)
	}
	if err := conn.Send("HSET", db2.MLEventConfig+":name", modelDbKey, modelDbKey); err != nil {
		return fmt.Errorf("failed to queue HSET: %v", err)
	}

	return nil
}

func buildEventConfigKey(mlAlgorithm string, mlModelConfigName string, mlEventName string) string {
	return db2.MLEventConfig + "|" + mlAlgorithm + "|" + mlModelConfigName + "|" + mlEventName
}

func deleteMLEventConfigByName(conn redis.Conn, lc logger.LoggingClient, mlAlgorithmName string, mlModelConfigName string, eventName string) hedgeErrors.HedgeError {
	modelDbKey := buildEventConfigKey(mlAlgorithmName, mlModelConfigName, eventName)
	errorMessage := fmt.Sprintf("Error deleting ML event config: %v", eventName)

	// Queue the deletion commands as part of the existing transaction
	if err := conn.Send("DEL", modelDbKey); err != nil {
		lc.Errorf("failed to queue DEL: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if err := conn.Send("SREM", db2.MLEventConfig, modelDbKey); err != nil {
		lc.Errorf("failed to queue SREM: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if err := conn.Send("HDEL", db2.MLEventConfig+":name", modelDbKey); err != nil {
		lc.Errorf("failed to queue HDEL: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DBClient) GetMLEventConfigByName(mlAlgorithmName string, mlModelConfigName string, mlEventName string) (config.MLEventConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	modelDbKey := buildEventConfigKey(mlAlgorithmName, mlModelConfigName, mlEventName)

	errorMessage := fmt.Sprintf("Error getting ml event config by name %s", modelDbKey)

	var eventConfig config.MLEventConfig
	err := redis2.GetObjectById(conn, modelDbKey, unmarshalObject, &eventConfig)

	if err != nil {
		lc.Errorf("Error While Get ML Event Config from DB for key: %s, %v", modelDbKey, err)
		if errors.Is(err, db2.ErrNotFound) {
			return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "ML Event Config not found")
		}

		return config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	return eventConfig, nil
}

func (dbClient *DBClient) GetAllMLEventConfigsByConfig(mlAlgorithmName, mlModelConfigName string) ([]config.MLEventConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting all ml event configs by config %s algorithm %s", mlModelConfigName, mlAlgorithmName)

	// Construct the pattern with a wildcard for mlEventName
	keyPattern := fmt.Sprintf("%s|%s|%s|*", db2.MLEventConfig, mlAlgorithmName, mlModelConfigName)
	lc.Infof("GetAllMLEventConfigsByConfig: key pattern: %s", keyPattern)

	objects, err := redis2.GetObjectsByPattern(conn, keyPattern)
	if err != nil {
		lc.Errorf("error while getting all ML event configs by ML model config from DB: %v", err)
		if errors.Is(err, db2.ErrNotFound) {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage)
		}

		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	configs := make([]config.MLEventConfig, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &configs[i])
		if err != nil {
			lc.Errorf("%s: Error While Unmarshal ML Event Config from DB: %v", errorMessage, err)
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return configs, nil
}

func (dbClient *DBClient) DeleteMLEventConfigByName(mlAlgorithmName string, mlModelConfigName string, eventName string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	return deleteMLEventConfigByName(conn, lc, mlAlgorithmName, mlModelConfigName, eventName)
}

func (dbClient *DBClient) SaveMLModelConfig(mlModelConfig config.MLModelConfig) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error saving ml model config: %v", mlModelConfig.Name)

	defer conn.Close()

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.MLAlgorithm+":name", mlModelConfig.MLAlgorithm)) //HEXISTS hx:ml:algo:name HedgeAnomaly
	if err != nil {
		lc.Errorf("%s: Error checking existence of ML Model Config: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error saving ml model config: %v", mlModelConfig.Name))
	} else if !exists {
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s:%s", errorMessage, "Not Found"))
	}

	return addTrainingConfig(conn, lc, mlModelConfig)
}

func addTrainingConfig(conn redis.Conn, lc logger.LoggingClient, mlModelConfig config.MLModelConfig) (string, hedgeErrors.HedgeError) {
	trainingDataKey := buildTrainingConfigKeyForTrngCfg(mlModelConfig.MLAlgorithm, mlModelConfig.Name)

	errorMessage := fmt.Sprintf("Error adding training config: %v", mlModelConfig.Name)

	m, err := marshalObject(mlModelConfig)
	if err != nil {
		lc.Errorf("Error marshalling ML model config: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	baseKeyByAlgo := db2.MLModelConfig + "|" + mlModelConfig.MLAlgorithm
	baseKeyWithoutAlgo := db2.MLModelConfig

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", trainingDataKey, m) // Should be algoName+trainingConfigName
	_ = conn.Send("SADD", baseKeyByAlgo, trainingDataKey)
	_ = conn.Send("SADD", baseKeyWithoutAlgo, trainingDataKey) // This is to enable get across all trainingConfigs
	_ = conn.Send("HSET", baseKeyByAlgo+":name", trainingDataKey, trainingDataKey)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error saving general training config in DB: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return mlModelConfig.Name, nil
}

func buildTrainingConfigKeyForTrngCfg(mlAlgorithm string, trainingConfigName string) string {
	return db2.MLModelConfig + "|" + mlAlgorithm + "|" + trainingConfigName
}

func (dbClient *DBClient) GetMlModelConfig(mlAlgorithm string, trainingConfigName string) (config.MLModelConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var mlModelConfig config.MLModelConfig

	trainingConfigKey := buildTrainingConfigKeyForTrngCfg(mlAlgorithm, trainingConfigName)

	err := redis2.GetObjectById(conn, trainingConfigKey, unmarshalObject, &mlModelConfig)
	if err != nil {
		lc.Errorf("Error getting training config from db: %s", err.Error())
		if errors.Is(err, db2.ErrNotFound) {
			return mlModelConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "ML model config not found")
		} else {
			return mlModelConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error getting ML model config")
		}
	} else {
		mlModelConfig.TrainedModelCount = dbClient.trainedModelExist(mlAlgorithm, trainingConfigName, conn)
	}

	return mlModelConfig, nil
}

func (dbClient *DBClient) GetAllMLModelConfigs(mlAlgorithm string) ([]config.MLModelConfig, hedgeErrors.HedgeError) {

	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error getting all ML model configs for algorithm: %s", mlAlgorithm)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	baseKeyWithOrWithoutAlgo := db2.MLModelConfig
	if mlAlgorithm != "" {
		baseKeyWithOrWithoutAlgo += "|" + mlAlgorithm
	}

	objects, err := redis2.GetObjectsByValue(conn, baseKeyWithOrWithoutAlgo)
	if err != nil {
		lc.Errorf("Error getting ML Training configurations from DB: %s", err.Error())
		return []config.MLModelConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]config.MLModelConfig, 0)
	for _, object := range objects {
		if object != nil {
			mlModelConfig := new(config.MLModelConfig)
			err = unmarshalObject(object, &mlModelConfig)
			if err != nil {
				//return []config.MLModelConfig{}, err
				// At least continue for other training configs that are goods
				lc.Errorf("Error unmarshaling training data :error: %s, object: %v", err.Error(), object)
				continue
			}

			//trgConfig.TrainedModelCount = dbClient.trainedModelExist(trgConfig.MLAlgorithm, trgConfig.Name, conn)

			d = append(d, *mlModelConfig)
		}
	}

	return d, nil
}

func (dbClient *DBClient) trainedModelExist(mlAlgorithm string, trainingConfigName string, conn redis.Conn) int {
	trgModelKey := buildModelBaseKey(mlAlgorithm, trainingConfigName)
	ids, err := redis.Values(conn.Do("SMEMBERS", trgModelKey))
	if err == nil && len(ids) > 0 {
		return len(ids)
	} else {
		return 0
	}
}

func (dbClient *DBClient) DeleteMLModelConfig(mlAlgorithm string, trainingDataConfig string) hedgeErrors.HedgeError {

	lc := dbClient.client.Logger
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	baseKeyByAlgo := db2.MLModelConfig + "|" + mlAlgorithm
	trainingConfigKey := buildTrainingConfigKeyForTrngCfg(mlAlgorithm, trainingDataConfig)
	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", trainingConfigKey)
	_ = conn.Send("SREM", baseKeyByAlgo, trainingConfigKey)
	_ = conn.Send("SREM", db2.MLModelConfig, trainingConfigKey)
	_ = conn.Send("HDEL", baseKeyByAlgo+":name", trainingConfigKey)

	_, err := conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error deleting general training job in DB: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error deleting ML model config for algorithm %s", mlAlgorithm))
	}

	return nil
}
