/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"hedge/common/db"
	commonRedis "hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/gomodule/redigo/redis"
)

type SyncDBClient commonRedis.DBClient

func NewDBClient(dbConfig *db.DatabaseConfig, logger logger.LoggingClient) DBClient {
	dbc := commonRedis.CreateDBClient(dbConfig)
	dbc.Logger = logger
	return (*SyncDBClient)(dbc)
}

type DBClient interface {
	GetState() ([]byte, hedgeErrors.HedgeError)
	SetState([]byte) hedgeErrors.HedgeError

	GetStateCoreVersion() string
	SetStateCoreVersion(string) hedgeErrors.HedgeError

	KnownNode(string) string
	SetKnownNode(string, string) hedgeErrors.HedgeError
}

func (dbClient *SyncDBClient) GetState() ([]byte, hedgeErrors.HedgeError) {
	// Get a connection from the pool
	conn := dbClient.Pool.Get()
	defer conn.Close()

	// Perform the GET operation
	jsonBytes, err := redis.Bytes(conn.Do("GET", "ms:sync-state"))
	if err != nil {
		dbClient.Logger.Errorf("failed to get state: %s", err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Failed to get state")
	}

	return jsonBytes, nil
}

func (dbClient *SyncDBClient) SetState(st []byte) hedgeErrors.HedgeError {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", "ms:sync-state", st)
	if err != nil {
		dbClient.Logger.Errorf("failed to set node state: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Failed to set state") // or handle error appropriately
	}

	return nil
}

func (dbClient *SyncDBClient) GetStateCoreVersion() string {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	val, err := redis.String(conn.Do("GET", "ms:core-sync-state-version"))
	if err != nil {
		dbClient.Logger.Errorf("failed to retrieve last core state version: %s", err.Error())
		return "" // or handle error appropriately
	}

	return val
}

func (dbClient *SyncDBClient) SetStateCoreVersion(version string) hedgeErrors.HedgeError {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", "ms:core-sync-state-version", version)
	if err != nil {
		dbClient.Logger.Errorf("failed to set core state version: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Failed to set state core version "+version)
	}

	return nil
}

func (dbClient *SyncDBClient) KnownNode(node string) string {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	val, err := redis.String(conn.Do("GET", fmt.Sprintf("ms:sync-known-node-%s", node)))
	if err != nil {
		dbClient.Logger.Errorf("failed to retrieve known host: %s", err.Error())
		return ""
	}

	return val
}

func (dbClient *SyncDBClient) SetKnownNode(node string, host string) hedgeErrors.HedgeError {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", fmt.Sprintf("ms:sync-known-node-%s", node), host)
	if err != nil {
		dbClient.Logger.Errorf("failed to set known host: %v", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Failed to set host %s on node %s", host, node))
	}
	return nil
}
