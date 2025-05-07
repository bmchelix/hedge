/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	hedgeErrors "hedge/common/errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"github.com/gomodule/redigo/redis"
)

func (dbClient *DBClient) UpdateModelDeployment(modelDeployment ml_model.ModelDeploymentStatus) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	deplyStatus := modelDeployment.DeploymentStatusCode
	modelDeployment.DeploymentStatus = modelDeployment.DeploymentStatusCode.String()
	if deplyStatus == ml_model.PublishedDeployCommand || deplyStatus == ml_model.PublishedUnDeployCommand {
		// PublishedDeploy or PublisedUndeploy ie in process
		modelDeployment.PermittedOption = "none"
	} else if deplyStatus == ml_model.ModelDeployed || deplyStatus == ml_model.ModelUnDeploymentFailed {
		// Deployed or UnDeployFailed, so allow undeploy, deploy both
		modelDeployment.PermittedOption = "all"
	} else if !modelDeployment.IsModelDeprecated {
		// as long as not deprecated, allow deploy
		modelDeployment.PermittedOption = "deploy"
	} else {
		modelDeployment.PermittedOption = "none"
	}

	return updateModelDeployment(conn, lc, modelDeployment)
}

// Updates the model deployment for each node in the database
// hx:ml:depstat -> All deployments
// hx:ml:depstat:node|node-1-> All deployments by node
// hx:ml:depstat:config|trgConfig-> All deployments by config
//

func updateModelDeployment(conn redis.Conn, lc logger.LoggingClient, modelDeploy ml_model.ModelDeploymentStatus) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error updating model deployment %s", modelDeploy.ModelName)

	deploymentKey := buildDeploymentKey(modelDeploy.MLAlgorithm, modelDeploy.MLModelConfigName, modelDeploy.NodeId)

	modelConfigKey := buildDeploymentConfigKey(modelDeploy.MLAlgorithm, modelDeploy.MLModelConfigName)
	/*	exists, err := redis.Bool(conn.Do("HEXISTS", db2.MLModelDeploymentStatus+":name", key))
		if err != nil {
			return err
		} else if exists {
			return db2.ErrNotUnique
		}*/

	m, err := marshalObject(modelDeploy)
	if err != nil {
		lc.Errorf("Error marshalling model deployment status to bytes: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	modelNameVersion := buildModelVersionKey(modelDeploy.MLAlgorithm, modelDeploy.MLModelConfigName, modelDeploy.ModelVersion)
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", deploymentKey, m)
	//All Deployments
	_ = conn.Send("SADD", db2.MLDeployment, deploymentKey)
	//modelConfigKey=db2.MLDeployment + ":config:" + modelDeploy.MLAlgorithm + "|" + modelDeploy.MLModelConfigName
	_ = conn.Send("SADD", modelConfigKey, deploymentKey) // Each element is for config+node combination, so for byConfig
	//All deployments by NodeName
	_ = conn.Send("SADD", db2.MLDeployment+":node"+"|"+modelDeploy.NodeId, deploymentKey)
	// To support query by modelNameVersion
	//All deployments by NodeName
	_ = conn.Send("SADD", db2.MLDeployment+":modelver"+"|"+modelNameVersion, deploymentKey)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error saving ML training job in db: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func buildModelVersionKey(mlAlgorithmName string, mlModelConfigName string, modelVersion int64) string {
	return fmt.Sprintf("%s|%s|%d", mlAlgorithmName, mlModelConfigName, modelVersion)
}

func (dbClient *DBClient) GetDeploymentsByConfig(mlAlgorithm string, mlModelConfigName string) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger
	errorMessage := fmt.Sprintf("Error fetching deployments for ml algorithm %s", mlAlgorithm)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var key string
	if mlAlgorithm == "" || mlModelConfigName == "" {
		key = db2.MLDeployment
	} else {
		key = buildDeploymentConfigKey(mlAlgorithm, mlModelConfigName)
	}

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		lc.Errorf("Error getting deployed models from db: %v", err)
		return []ml_model.ModelDeploymentStatus{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]ml_model.ModelDeploymentStatus, 0)
	i := 0
	for _, object := range objects {
		var obj ml_model.ModelDeploymentStatus
		err = unmarshalObject(object, &obj)
		if err != nil {
			lc.Errorf("Error getting deployed models from db: %v", err)
			// Keep accumulating the good ones
			//return []ml_model.ModelDeploymentStatus{}, err
		} else {
			d = append(d, obj)
			i++
		}
	}

	return d, nil
}

func (dbClient *DBClient) GetDeploymentsByNode(nodeName string) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Error fetching deployments for node %s", nodeName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := db2.MLDeployment + ":node" + "|" + nodeName

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		lc.Errorf("Error getting deployed models from db: %v", err)
		return []ml_model.ModelDeploymentStatus{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]ml_model.ModelDeploymentStatus, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			return []ml_model.ModelDeploymentStatus{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return d, nil
}

// GetDeploymentsByModelVersion returns the deployedModels to nodes by ModelVersion,
// this will be used to find the nodes to which the MlEventConfig needs to be updated
func (dbClient *DBClient) GetDeploymentsByModelVersion(mlAlgorithmName string, mlModelConfigName string, modelVersion int64) ([]ml_model.ModelDeploymentStatus, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error fetching deployments for ml algorithm %s", mlAlgorithmName)

	modelNameVersion := buildModelVersionKey(mlAlgorithmName, mlModelConfigName, modelVersion)
	key := db2.MLDeployment + ":modelver" + "|" + modelNameVersion

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		lc.Errorf("Error getting deployed models from db by modelVersion: %v", err)
		return []ml_model.ModelDeploymentStatus{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]ml_model.ModelDeploymentStatus, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			lc.Errorf("%s: Error unmarshalling deployed models from db by modelVersion: %v", errorMessage, err)
			return []ml_model.ModelDeploymentStatus{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return d, nil
}

// GetDeploymentsByModelName
// getAllDeployments - In service

func buildDeploymentKey(mlAlgorithm string, mlModelConfigName string, nodeId string) string {
	//modelVersionStr := strconv.FormatInt(modelVersion, 10)
	return db2.MLDeployment + "|" + mlAlgorithm + "|" + mlModelConfigName + "|" + nodeId
}

func buildDeploymentConfigKey(mlAlgorithm string, mlModelConfigName string) string {
	return db2.MLDeployment + ":config" + "|" + mlAlgorithm + "|" + mlModelConfigName
}
