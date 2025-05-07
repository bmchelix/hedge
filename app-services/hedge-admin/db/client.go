/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	localModel "hedge/app-services/hedge-admin/models"
	"hedge/common/db"
	"hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	models2 "hedge/common/models"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type RedisDbClient interface {
	SaveProtocols(dsps models2.DeviceServiceProtocols) ([]string, hedgeErrors.HedgeError)
	SaveNode(node *models2.Node) (string, hedgeErrors.HedgeError)
	DeleteNode(nodeId string, keyFieldTuples []localModel.KeyFieldTuple) hedgeErrors.HedgeError
	GetNode(nodeName string) (*models2.Node, hedgeErrors.HedgeError)
	GetAllNodes() ([]models2.Node, hedgeErrors.HedgeError)
	SaveNodeGroup(parentGroup string, dbNodeGroup *models2.DBNodeGroup) (string, hedgeErrors.HedgeError)
	FindNodeKey(parentGroup string) (string, hedgeErrors.HedgeError)
	UpsertChildNodeGroups(parentNodeName string, childNodes []string) (string, hedgeErrors.HedgeError)
	GetNodeGroup(nodeGroupName string) (*models2.DBNodeGroup, hedgeErrors.HedgeError)
	GetDBNodeGroupMembers(nodeHashKey string) ([]models2.DBNodeGroup, hedgeErrors.HedgeError)
	DeleteNodeGroup(parentNodeGroupName string, field string) hedgeErrors.HedgeError
}

type DBClient redis.DBClient

func NewDBClient(service interfaces.ApplicationService) *DBClient {
	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(service)
	dbClient := redis.CreateDBClient(dbConfig)
	return (*DBClient)(dbClient)
}
