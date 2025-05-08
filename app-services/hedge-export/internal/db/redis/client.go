/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"hedge/common/db"
	"hedge/common/db/redis"
)

type DBClient redis.DBClient

func NewDBClient(dbConfig *db.DatabaseConfig) *DBClient {
	dbClient := redis.CreateDBClient(dbConfig)
	return (*DBClient)(dbClient)
}
