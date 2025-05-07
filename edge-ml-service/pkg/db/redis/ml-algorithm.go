package redis

import (
	"hedge/common/db"
	redis2 "hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"errors"
	"fmt"

	"hedge/edge-ml-service/pkg/dto/config"
	"github.com/gomodule/redigo/redis"
)

func (dbClient *DBClient) GetAlgorithm(algoName string) (*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error geting algorithm %s", algoName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := dbClient.algorithmKey(algoName)

	m, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		// not found
		if errors.Is(err, redis.ErrNil) {
			return nil, nil
		}
		lc.Errorf("Error getting algorithm definition: %s", err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var algo config.MLAlgorithmDefinition
	err = unmarshalObject(m, &algo)
	if err != nil {
		lc.Errorf("error while unmarshalling algorithm definition: %s", err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return &algo, nil
}

func (dbClient *DBClient) GetAllAlgorithms() ([]*config.MLAlgorithmDefinition, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger
	errorMessage := "Error getting all algorithms"

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	objects, err := redis2.GetObjectsByValue(conn, db.MLAlgorithm)
	if err != nil {
		lc.Errorf("Error getting general algorithms from DB: %v", err)
		return []*config.MLAlgorithmDefinition{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]*config.MLAlgorithmDefinition, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			lc.Errorf("Error unmarshalling algorithm definition: %v", err)
			return []*config.MLAlgorithmDefinition{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return d, nil
}

func (dbClient *DBClient) CreateAlgorithm(algo config.MLAlgorithmDefinition) hedgeErrors.HedgeError {

	errorMessage := fmt.Sprintf("Error creating algorithm %s", algo.Name)

	lc := dbClient.client.Logger
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := dbClient.algorithmKey(algo.Name)

	exists, err := redis.Bool(conn.Do("HEXISTS", db.MLAlgorithm+":name", algo.Name))
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if exists {
		lc.Errorf("%s: %v", errorMessage, fmt.Sprintf("Algorithm '%s' already exists", algo.Name))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Algorithm '%s' already exists", algo.Name))
	}

	m, err := marshalObject(algo)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", key, m)
	_ = conn.Send("SADD", db.MLAlgorithm, key)
	_ = conn.Send("HSET", db.MLAlgorithm+":name", algo.Name, algo.Name)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: error saving algorithm definition: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DBClient) UpdateAlgorithm(algo config.MLAlgorithmDefinition) hedgeErrors.HedgeError {

	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error updating algorithm %s", algo.Name)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := dbClient.algorithmKey(algo.Name)

	exists, err := redis.Bool(conn.Do("HEXISTS", db.MLAlgorithm+":name", algo.Name))
	if errors.Is(err, redis.ErrNil) {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Algorithm '%s' not found", algo.Name))
	}

	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if !exists {
		lc.Errorf("%s: %v", errorMessage, fmt.Sprintf("Algorithm '%s' not found", algo.Name))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Algorithm '%s' not found", algo.Name))
	}

	m, err := marshalObject(algo)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", key, m)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DBClient) DeleteAlgorithm(algoName string) hedgeErrors.HedgeError {
	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error deleting algorithm %s", algoName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := dbClient.algorithmKey(algoName)

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", key)
	_ = conn.Send("SREM", db.MLAlgorithm, key)
	_ = conn.Send("HDEL", db.MLAlgorithm+":name", algoName)

	_, err := conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DBClient) algorithmKey(name string) string {
	return db.MLAlgorithm + "|" + name
}
