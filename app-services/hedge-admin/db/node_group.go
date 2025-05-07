/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	"errors"

	localModel "hedge/app-services/hedge-admin/models"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	"encoding/json"
	"fmt"

	"hedge/common/models"
	"github.com/gomodule/redigo/redis"
)

// Storage Structure
// nodeGroup:node:name -> Stores the master data for nodes
// nodeGroup ->Stores a set of top level groups
// nodeGroup:groupName -> Stores a set of group elements
// nodeGroup:groupName:subgroup -> Stores a set of group elements

func (dbClient *DBClient) SaveNode(node *models.Node) (string, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	// Convert node to bytes and then add
	bytes, _ := json.Marshal(node)

	id := db2.Node + ":" + node.NodeId
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", id, bytes) // Not right, need to have db2.Node prefix
	_ = conn.Send("SADD", db2.Node, id)
	_ = conn.Send("HSET", db2.Node+":name", node.NodeId, id)
	_ = conn.Send("HSET", db2.Node+":host", node.HostName, id)

	_, err := conn.Do("EXEC")
	if err != nil {
		dbClient.Logger.Errorf("Error saving node %s in DB: %v", node.NodeId, err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			"Error saving node "+node.NodeId,
		)
	}

	// Check with below command in redis-cli
	// HGET hx:nodeGroup:<groupName> FurnacePlant
	return id, nil
}

func (dbClient *DBClient) DeleteNode(
	nodeId string,
	keyFieldTuples []localModel.KeyFieldTuple,
) hedgeErrors.HedgeError {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	node, err := dbClient.GetNode(nodeId)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		return err
	}

	id := db2.Node + ":" + nodeId
	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", id)
	_ = conn.Send("SREM", db2.Node, nodeId)
	_ = conn.Send("HDEL", db2.Node+":name", nodeId)

	// Below is just to clean the old data, not required
	_ = conn.Send("DEL", nodeId)

	if node != nil {
		_ = conn.Send("HDEL", db2.Node+":host", node.HostName)
	}

	// Delete all immediate holding container group
	for _, tuple := range keyFieldTuples {
		_ = conn.Send("HDEL", tuple.Key, tuple.Field)
	}

	_, _err := conn.Do("EXEC")
	if _err != nil {
		dbClient.Logger.Errorf("Error deleting node in DB: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error deleting node")
	}

	return nil
}
func (dbClient *DBClient) GetNode(nodeName string) (*models.Node, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	var node models.Node
	id := db2.Node + ":" + nodeName
	err := redis2.GetObjectById(conn, id, unmarshalObject, &node)

	if errors.Is(err, db2.ErrNotFound) {
		// Assume the nodeName passed in the hostName, so search by hostName
		// var deviceExt []dto2.DeviceExt = make([]dto2.DeviceExt, 0)
		err := redis2.GetObjectByKey(conn, db2.Node+":host", nodeName, &node)
		if errors.Is(err, db2.ErrNotFound) {
			dbClient.Logger.Errorf("Error getting node %s in DB: %v", nodeName, err)
			return nil, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("Node %s not found", nodeName),
			)
		}
		if err != nil {
			dbClient.Logger.Warnf("Error getting node by hostName: %v", err)
			return nil, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("Error getting node %s by hostname", nodeName),
			)
		}
	} else if err != nil {
		dbClient.Logger.Errorf("Error getting node %s in DB: %v", nodeName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error getting node %s", nodeName))
	}

	return &node, nil
}

func (dbClient *DBClient) GetAllNodes() ([]models.Node, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := "Error getting all nodes"

	objects, err := redis2.GetObjectsByValue(conn, db2.Node)
	if err != nil {
		dbClient.Logger.Errorf("Error getting general nodes from DB: objects not found: %v", err)
		return []models.Node{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError,
			errorMessage)
	}

	d := make([]models.Node, len(objects))
	i := 0
	for _, object := range objects {
		if object == nil {
			continue
		}
		err = json.Unmarshal(object, &d[i])
		i += 1
		if err != nil {
			dbClient.Logger.Errorf("Error getting all nodes: unmarshaling error: %v", err)
			return []models.Node{}, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				errorMessage,
			)
		}
	}

	return d, nil
}

func (dbClient *DBClient) SaveNodeGroup(
	parentGroup string,
	dbNodeGroup *models.DBNodeGroup,
) (string, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error saving node group of parent %s", parentGroup)

	// Make sure the node exists if it is specified
	if dbNodeGroup.NodeId != "" {
		exists, err := redis.Bool(conn.Do("HEXISTS", db2.Node+":name", dbNodeGroup.NodeId))
		if err != nil {
			dbClient.Logger.Errorf(
				"Error saving node group %s: DB error %v",
				dbNodeGroup.NodeId,
				err,
			)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
		if !exists {
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("Node group %s not found", dbNodeGroup.NodeId),
			)
		}
	}

	var nodeHashKey string
	var err error
	if parentGroup == "" {
		nodeHashKey = db2.NodeGroup
	} else {
		nodeHashKey, err = findNodeGroup(conn, dbClient.Logger, db2.NodeGroup, parentGroup)
		if err != nil {
			dbClient.Logger.Errorf("Error saving node group %s: %v", dbNodeGroup.NodeId, err)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
		if nodeHashKey == "" {
			// parentNode doesn't exist, so not found error
			dbClient.Logger.Errorf("Error saving node group %s: %v", dbNodeGroup.NodeId, "parentNode doesn't exist")
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Node group not found")
		}
	}

	// Insert or update now
	// Convert nodeGroup to bytes and then add
	nodeKey := dbNodeGroup.Name
	bytes, _ := json.Marshal(dbNodeGroup)
	_, err = conn.Do("HSET", nodeHashKey, nodeKey, bytes)
	if err != nil {
		dbClient.Logger.Errorf("Error saving node group %s: %v", dbNodeGroup.NodeId, err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("Error saving node group %s: %v", dbNodeGroup.NodeId, err),
		)
	}
	// Check with below command in redis-cli
	// HGET hx:nodeGroup:<groupName> FurnacePlant
	return nodeHashKey, nil
}

func (dbClient *DBClient) FindNodeKey(parentGroup string) (string, hedgeErrors.HedgeError) {
	if parentGroup == "" {
		return db2.NodeGroup, nil
	}
	conn := dbClient.Pool.Get()
	defer conn.Close()
	return findNodeGroup(conn, dbClient.Logger, db2.NodeGroup, parentGroup)
}

func findNodeGroup(
	conn redis.Conn,
	logger logger.LoggingClient,
	hashKey string,
	parentGroup string,
) (string, hedgeErrors.HedgeError) {

	replies, err := conn.Do("HKEYS", hashKey)
	if err != nil || replies == nil {
		logger.Errorf(
			"Error getting node group of parent %s from DB by hashKey: %v",
			parentGroup,
			err,
		)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			"Find node group failed",
		)
	}
	// replies in this case is of type []interface{}, so need to iterate it to convert to Byte array

	repliesList, _ := replies.([]interface{})
	for _, reply := range repliesList {
		nodeGroupName, _ := redis.String(reply, nil)
		fmt.Println(nodeGroupName)
		if nodeGroupName == parentGroup {
			return hashKey + ":" + parentGroup, nil
		}
	}

	for _, reply := range repliesList {
		nodeGroupName, _ := redis.String(reply, nil)
		foundKey, err := findNodeGroup(conn, logger, hashKey+":"+nodeGroupName, parentGroup)
		if err != nil {
			return "", err
		}
		if foundKey != "" {
			return foundKey, nil
		}
	}

	return "", nil
}

func (dbClient *DBClient) UpsertChildNodeGroups(
	parentNodeName string,
	childNodes []string,
) (string, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := "Error updating child node groups"

	// Make sure of the nodes do exist
	exist, err := redis.Bool(conn.Do("HEXISTS", db2.NodeGroup, parentNodeName))
	if err != nil {
		dbClient.Logger.Errorf(
			"Error upserting child node group for parent %s: Error getting node group from DB: %v",
			parentNodeName,
			err,
		)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !exist {
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			fmt.Sprintf("Node group %s not found", parentNodeName),
		)
	}

	for _, childNode := range childNodes {
		exist, err = redis.Bool(conn.Do("HEXISTS", db2.NodeGroup, childNode))
		if err != nil {
			dbClient.Logger.Errorf(
				"Error upserting child node group for parent %s: Node group not found in the DB: %v",
				parentNodeName,
				err,
			)
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
		if !exist {
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("Child node %s not found", childNode),
			)
		}
	}

	childKey := buildChildNodesKey(parentNodeName)

	// Hopefully, no need to convert to bytes etc
	bytes, _ := json.Marshal(childNodes)
	_, err = conn.Do("SADD", childKey, bytes)
	if err != nil {
		dbClient.Logger.Errorf(
			"Error upserting child node group for parent %s: DB error: %v",
			parentNodeName,
			err,
		)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Error updating child node groups",
		)
	}
	return childKey, nil
}

func (dbClient *DBClient) GetNodeGroup(
	nodeGroupName string,
) (*models.DBNodeGroup, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting node group for %s", nodeGroupName)

	object, err := redis.Bytes(conn.Do("HGET", db2.NodeGroup, nodeGroupName))
	if errors.Is(err, redis.ErrNil) {
		dbClient.Logger.Errorf(
			"Error getting node group %s: Node group not found in DB: %v",
			nodeGroupName,
			err,
		)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			fmt.Sprintf("Node group %s not found", nodeGroupName),
		)
	} else if err != nil {
		dbClient.Logger.Errorf("Error getting node group %s: DB Error: %v", nodeGroupName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var dNodeGroup *models.DBNodeGroup
	err = json.Unmarshal(object, dNodeGroup)
	if err != nil {
		dbClient.Logger.Errorf(
			"Error getting node group %s: Unmarshalling error %v",
			nodeGroupName,
			err,
		)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	return dNodeGroup, nil
}

func (dbClient *DBClient) GetDBNodeGroupMembers(
	nodeHashKey string,
) ([]models.DBNodeGroup, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := "Error getting node group members"

	objects, err := conn.Do("HVALS", nodeHashKey)
	if errors.Is(err, redis.ErrNil) {
		dbClient.Logger.Errorf(
			"Error getting db node group members for %s: Error getting values from DB: %v",
			nodeHashKey,
			err,
		)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeNotFound,
			"Node group members not found",
		)
	} else if err != nil {
		dbClient.Logger.Errorf("Error getting db node group members for %s: Error getting values from DB: %v", nodeHashKey, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	objectList, _ := objects.([]interface{})
	childDbNodeGroups := make([]models.DBNodeGroup, len(objectList))
	for i, object := range objectList {
		bytes, err := redis.Bytes(object, nil)
		if err != nil {
			dbClient.Logger.Errorf(
				"Error getting db node group members for %s: %v",
				nodeHashKey,
				err,
			)
			return nil, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				errorMessage,
			)
		}
		var dbNodeGrp models.DBNodeGroup
		err = json.Unmarshal(bytes, &dbNodeGrp)
		if err != nil {
			dbClient.Logger.Errorf(
				"Error getting db node group members for %s: Unmarshalling error:  %v",
				nodeHashKey,
				err,
			)
			return nil, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				errorMessage,
			)
		}
		childDbNodeGroups[i] = dbNodeGrp
	}

	return childDbNodeGroups, nil
}

func buildChildNodesKey(parentNodeName string) string {
	childKey := db2.NodeGroup + ":" + parentNodeName
	return childKey
}

// DeleteNodeGroup Deletes the nodeGroup ( hx:nodeGroup:GroupName field)
func (dbClient *DBClient) DeleteNodeGroup(
	parentNodeGroupName string,
	field string,
) hedgeErrors.HedgeError {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	_, err := conn.Do("HDEL", parentNodeGroupName, field)
	if err != nil {
		dbClient.Logger.Errorf(
			"Error deleting node group %s: Error deleting values from DB: %v",
			parentNodeGroupName,
			err,
		)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeDBError,
			"Error deleting node group",
		)
	}
	return nil
}

func unmarshalObject(in []byte, out interface{}) (err error) {
	return json.Unmarshal(in, out)
}
