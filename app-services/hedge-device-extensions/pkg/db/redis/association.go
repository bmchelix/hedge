/*******************************************************************************
 * Copyright 2018 Redis Labs Inc.
 * (c) Copyright 2020-2025 BMC Software, Inc.
 *
 * Contributors: BMC Software, Inc. - BMC Helix Edge
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package redis

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
)

func (dbClient *DeviceExtDBClient) AddAssociation(nodeA string, nodeAType string, associationNode []dto.AssociationNode) (string, hedgeErrors.HedgeError) {
	dbClient.client.Logger.Debugf("Adding association to node %s %v", nodeA, associationNode)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	return dbClient.addAssociation(conn, nodeA, nodeAType, associationNode)
}

func (dbClient *DeviceExtDBClient) addAssociation(conn redis.Conn, nodeA string, nodeAType string, associationNode []dto.AssociationNode) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error adding association for node %s", nodeA)

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.Association+":name", nodeA))
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if exists {
		dbClient.client.Logger.Errorf("%s: Not Unique", errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: Association already exists", errorMessage))
	}

	dexists, err := redis.Bool(conn.Do("HEXISTS", db2.Device+":name", nodeA))
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if !dexists {
		dbClient.client.Logger.Errorf("%s: Not Found", errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device not Found", errorMessage))
	}

	m, err := marshalObject(associationNode)
	uid := uuid.New().String()
	id := db2.Association + ":" + uid
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", id, m)                               // Store payload against ID
	_ = conn.Send("HSET", db2.Association+":name", nodeA, id) // Map the id with nodeName
	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("Error saving association to DataBase: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("%s: failed to save association", errorMessage))
	}

	return uid, nil
}

func (dbClient *DeviceExtDBClient) GetAssociationByNodeName(node string) ([]dto.AssociationNode, hedgeErrors.HedgeError) {
	dbClient.client.Logger.Debugf("Getting association by node name %s", node)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var associations []dto.AssociationNode

	err := redis2.GetObjectByKey(conn, db2.Association+":name", node, &associations)
	if errors.Is(err, db2.ErrNotFound) {
		dbClient.client.Logger.Debugf("Association does not exist for %s. Return empty.", node)
		return associations, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Association does not exist for "+node)
	} else if err != nil {
		dbClient.client.Logger.Debugf("Error Getting Association from DataBase: %v", err)
		return associations, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error getting association for node "+node)
	}

	return associations, nil
}

func (dbClient *DeviceExtDBClient) DeleteAssociationByNodeName(node string) hedgeErrors.HedgeError {
	dbClient.client.Logger.Debugf("Deleting association by node name %s", node)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	err := dbClient.deleteAssociationByNodeName(conn, node)
	return err
}

func (dbClient *DeviceExtDBClient) deleteAssociationByNodeName(conn redis.Conn, node string) hedgeErrors.HedgeError {
	id, err := redis.String(conn.Do("HGET", db2.Association+":name", node))
	if err != nil {
		dbClient.client.Logger.Errorf("Node not found while getting association %s from DataBase", node)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Node not found while trying to delete association %s", node))
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", id) // Store payload against ID
	_ = conn.Send("HDEL", db2.Association+":name", node)
	// _ = conn.Send("HSET", db.Association+":name", nodeA, id) // Map the id with nodeName
	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting association in DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error deleting association for node "+node)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) UpdateAssociation(nodeA string, nodeAType string, associationNode []dto.AssociationNode) (string, hedgeErrors.HedgeError) {
	dbClient.client.Logger.Debugf("Updating association node %s node type %s, associationNode %v", nodeA, nodeAType, associationNode)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error updating association for node %s; node Type %s", nodeA, nodeAType)

	err := dbClient.deleteAssociationByNodeName(conn, nodeA)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", err
	}

	association, err := dbClient.addAssociation(conn, nodeA, nodeAType, associationNode)

	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", err
	}

	return association, nil
}
