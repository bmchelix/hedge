/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
)

func (dbClient *DeviceExtDBClient) AddLocation(location models.Location) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	return dbClient.addLocation(conn, location)
}

func (dbClient *DeviceExtDBClient) addLocation(conn redis.Conn, location models.Location) (string, hedgeErrors.HedgeError) {
	errorMesssage := fmt.Sprintf("Error adding location %s", location.DisplayName)

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.Location+":name", location.DisplayName))
	if err != nil {
		dbClient.client.Logger.Errorf("%s: location id %s %v", errorMesssage, location.Id, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMesssage)
	} else if exists {
		dbClient.client.Logger.Errorf("%s location id %s: Already exists", location.Id, errorMesssage)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: Already exists", errorMesssage))
	}

	_, err = uuid.Parse(location.Id)
	if err != nil {
		location.Id = uuid.New().String()
	}
	id := location.Id

	m, err := marshalObject(location)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMesssage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMesssage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", id, m)
	_ = conn.Send("ZADD", db2.Location, 0, id)
	_ = conn.Send("HSET", db2.Location+":name", location.DisplayName, id)
	_ = conn.Send("SADD", db2.Location+":state:"+location.State, id)
	_ = conn.Send("SADD", db2.Location+":country:"+location.Country, id)
	_ = conn.Send("SADD", db2.Location+":city:"+location.City, id)

	_, err = conn.Do("EXEC")
	if err != nil {
		dbClient.client.Logger.Errorf("%s: Error Saving Location in DB: location id: %s  %v", errorMesssage, location.Id, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("%s: failed to save location", errorMesssage))
	}

	return id, nil
}

func (dbClient *DeviceExtDBClient) GetLocationById(id string) (models.Location, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMesssage := fmt.Sprintf("Error getting location %s", id)

	var location models.Location
	err := redis2.GetObjectById(conn, id, unmarshalObject, &location)
	if err != nil {
		dbClient.client.Logger.Errorf("Error Getting Location from DB: %s", err.Error())
		if errors.Is(err, db2.ErrNotFound) {
			return location, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Not found", errorMesssage))
		}

		return location, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMesssage)
	}

	return location, nil
}

func (dbClient *DeviceExtDBClient) GetLocations(key string) ([]models.Location, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting locations by key %s", key)

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: Error Getting Locations from DB: %v", errorMessage, err)
		if errors.Is(err, db2.ErrNotFound) {
			return []models.Location{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage+": Not Found")
		}

		return []models.Location{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]models.Location, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
			return []models.Location{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
	}

	return d, nil
}
