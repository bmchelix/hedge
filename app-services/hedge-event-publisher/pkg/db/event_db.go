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
	"hedge/common/db"
	"hedge/common/db/redis"
	comModels "hedge/common/models"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	redis2 "github.com/gomodule/redigo/redis"
)

type EventDBClient redis.DBClient

type DBClient interface {
	GetDbClient(dbConfig *db.DatabaseConfig) DBClient
	SaveEvent(event comModels.HedgeEvent) error
	GetEventByCorrelationId(correlationId string, lc logger.LoggingClient) (*comModels.HedgeEvent, error)
	DeleteEvent(correlationId string) error
}

var DBClientImpl DBClient

func init() {
	DBClientImpl = &EventDBClient{}
}

func NewDBClient(dbConfig *db.DatabaseConfig) DBClient {
	return DBClientImpl.GetDbClient(dbConfig)
}

func (dbClient *EventDBClient) GetDbClient(dbConfig *db.DatabaseConfig) DBClient {
	dbc := redis.CreateDBClient(dbConfig)
	return (*EventDBClient)(dbc)
}

func (dbClient *EventDBClient) SaveEvent(event comModels.HedgeEvent) error {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	evJson, _ := json.Marshal(event)
	_, err := conn.Do("HSET", db.OTEvent, event.CorrelationId, evJson)
	return err
}

func (dbClient *EventDBClient) GetEventByCorrelationId(correlationId string, lc logger.LoggingClient) (*comModels.HedgeEvent, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	var event comModels.HedgeEvent
	eventData, err := redis2.Bytes(conn.Do("HGET", db.OTEvent, correlationId))
	if err != nil && errors.Is(err, redis2.ErrNil) {
		lc.Infof("no existing event found for correlationId: %s, error: %s", correlationId, err.Error())
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	err = json.Unmarshal(eventData, &event)
	return &event, err

}

func (dbClient *EventDBClient) DeleteEvent(correlationId string) error {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	_, err := conn.Do("HDEL", db.OTEvent, correlationId)
	return err
}
