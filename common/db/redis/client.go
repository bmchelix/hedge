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
	"os"
	"sync"
	"time"

	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/startup"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/redigo"
	"github.com/gomodule/redigo/redis"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
)

var currClient *DBClient // a singleton so Readings can be de-referenced
var once sync.Once

// DBClient represents a Redis client
type DBClient struct {
	Pool      *redis.Pool // A thread-safe pool of connections to Redis
	BatchSize int
	Logger    logger.LoggingClient // to be initialized by DBClient consumer if needed for logging
	Client    CommonRedisDBInterface
	RedisSync *redsync.Redsync
}

type CommonRedisDBInterface interface {
	IncrMetricCounterBy(key string, value int64) (int64, hedgeErrors.HedgeError)
	GetMetricCounter(key string) (int64, hedgeErrors.HedgeError)
	SetMetricCounter(key string, value int64) hedgeErrors.HedgeError
	AcquireRedisLock(lockName string) (*redsync.Mutex, hedgeErrors.HedgeError)
}

func (c *DBClient) IncrMetricCounterBy(key string, value int64) (int64, hedgeErrors.HedgeError) {
	conn := c.Pool.Get()
	defer conn.Close()

	errorMessage := "Error incrementing metric counter"

	val, err := redis.Int64(conn.Do("INCRBY", key, value))
	if err != nil {
		c.Logger.Errorf("%s key %s value %v: %v", errorMessage, key, value, err)
		return 0, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else {
		return val, nil
	}
}

func (c *DBClient) SetMetricCounter(key string, value interface{}) hedgeErrors.HedgeError {
	conn := c.Pool.Get()
	defer conn.Close()

	errorMessage := "Error setting metric counter"

	_, err := conn.Do("SET", key, value)
	if err != nil {
		c.Logger.Errorf("%s key %s value %v: %v", errorMessage, key, value, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else {
		return nil
	}
}

func (c *DBClient) GetMetricCounter(key string) (int64, hedgeErrors.HedgeError) {
	conn := c.Pool.Get()
	defer conn.Close()

	errorMessage := "error getting metric"

	val, err := redis.Int64(conn.Do("GET", key))
	if errors.Is(err, redis.ErrNil) {
		return 0, nil // return 0 if key does not exist
	}
	if err != nil {
		c.Logger.Errorf("%s from DB for key %s: %v", errorMessage, key, err)
		return 0, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else {
		return val, nil
	}
}

func (c *DBClient) AcquireRedisLock(lockName string) (*redsync.Mutex, hedgeErrors.HedgeError) {
	mutex := c.RedisSync.NewMutex(lockName, redsync.WithExpiry(5*time.Second))

	// Retry logic for acquiring the lock
	for retries := 0; retries < 5; retries++ {
		if err := mutex.Lock(); err != nil {
			if retries == 4 {
				c.Logger.Errorf("Failed to acquire lock %s in Redis after multiple attempts: %v", lockName, err)
				return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to acquire lock in Redis after multiple attempts")
			}
			time.Sleep(1 * time.Second) // wait and retry
			continue
		}
		return mutex, nil
	}

	return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to acquire lock in Redis")
}

func CreateDBClient(dbConfig *db.DatabaseConfig) *DBClient {
	var dbClient *DBClient
	var err error
	startupTimer := startup.NewStartUpTimer("redis-db")
	for startupTimer.HasNotElapsed() {

		dbClient, err = newDBClient(dbConfig)
		if err == nil {
			break
		}
		dbClient = nil
		fmt.Printf("%s\n", fmt.Sprintf("Couldn't create database client: %v", err.Error()))
		startupTimer.SleepForInterval()
	}
	if dbClient == nil {
		fmt.Printf("%s\n", "Failed to create database client in allotted time")
		os.Exit(1)
	}
	return dbClient
}

// Return a pointer to the Redis client
func newDBClient(dbConfig *db.DatabaseConfig) (*DBClient, error) {
	once.Do(func() {
		connectionString := fmt.Sprintf("%s:%s", dbConfig.RedisHost, dbConfig.RedisPort)
		opts := []redis.DialOption{
			redis.DialConnectTimeout(time.Duration(9000) * time.Millisecond),
		}
		if os.Getenv("EDGEX_SECURITY_SECRET_STORE") != "false" {
			opts = append(opts, redis.DialPassword(dbConfig.RedisPassword))
		}

		dialFunc := func() (redis.Conn, error) {
			conn, err := redis.Dial(
				"tcp", connectionString, opts...,
			)
			if err != nil {
				return nil, fmt.Errorf("could not dial Redis: %s", err)
			}
			return conn, nil
		}
		// Default the batch size to 1,000 if not set
		batchSize := 1000
		// if dbConfig.BatchSize != 0 {
		// 	batchSize = dbConfig.BatchSize
		// }

		pool := &redis.Pool{
			IdleTimeout: 0,
			/* The current implementation processes nested structs using concurrent connections.
			 * With the deepest nesting level being 3, three shall be the number of maximum open
			 * idle connections in the pool, to allow reuse.
			 * TODO: Once we have a concurrent benchmark, this should be revisited.
			 * TODO: Longer term, once the objects are clean of external dependencies, the use
			 * of another serializer should make this moot.
			 */
			MaxIdle: 10,
			Dial:    dialFunc,
		}

		// Create Redsync instance
		redsyncInstance := redsync.New(redigo.NewPool(pool))
		currClient = &DBClient{
			Pool:      pool,
			BatchSize: batchSize,
			RedisSync: redsyncInstance,
		}
	})

	// Test connectivity now so don't have failures later when doing lazy connect.
	if _, err := currClient.Pool.Dial(); err != nil {
		return nil, err
	}

	return currClient, nil
}

// Connect connects to Redis
func (c *DBClient) Connect() error {
	return nil
}

// Do: Run a REDIS command
func (c *DBClient) Do(command string, args ...interface{}) (interface{}, error) {
	conn := c.Pool.Get()
	defer conn.Close()

	return conn.Do(command, args...)
}

// CloseSession closes the connections to Redis
func (c *DBClient) CloseSession() {
	_ = c.Pool.Close()
	currClient = nil
	once = sync.Once{}
}

// getConnection gets a connection from the pool
func GetClient() (*DBClient, error) {
	if currClient == nil {
		return nil, errors.New("no current Redis client: create a new client before getting a connection from it")
	}
	return currClient, nil
}
