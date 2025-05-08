/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package client

import (
	"github.com/go-redsync/redsync/v4"
	"hedge/common/db"
	"hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
)

type DBClient struct {
	client *redis.DBClient
}

var DBClientImpl DBClientInterface

type DBClientInterface interface {
	redis.CommonRedisDBInterface
	PublishToRedisBus(topic string, msg interface{}) error
	GetDbClient(dbConfig *db.DatabaseConfig) DBClientInterface
}

func (rc *DBClient) GetDbClient(dbConfig *db.DatabaseConfig) DBClientInterface {
	dbc := redis.CreateDBClient(dbConfig)
	return &DBClient{client: dbc}
}

func NewDBClient(dbConfig *db.DatabaseConfig) DBClientInterface {
	return DBClientImpl.GetDbClient(dbConfig)
}

func (rc *DBClient) PublishToRedisBus(topic string, msg interface{}) error {
	conn := rc.client.Pool.Get()
	defer conn.Close()
	_, err := conn.Do("PUBLISH", topic, msg)
	return err
}

func (rc *DBClient) IncrMetricCounterBy(key string, value int64) (int64, hedgeErrors.HedgeError) {
	return rc.client.IncrMetricCounterBy(key, value)
}

func (rc *DBClient) SetMetricCounter(key string, value int64) hedgeErrors.HedgeError {
	return rc.client.SetMetricCounter(key, value)
}

func (rc *DBClient) GetMetricCounter(key string) (int64, hedgeErrors.HedgeError) {
	return rc.client.GetMetricCounter(key)
}

func (rc *DBClient) AcquireRedisLock(name string) (*redsync.Mutex, hedgeErrors.HedgeError) {
	return rc.client.AcquireRedisLock(name)
}

func init() {
	DBClientImpl = &DBClient{}
}
