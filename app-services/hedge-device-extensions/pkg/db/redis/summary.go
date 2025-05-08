/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	contract "github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gomodule/redigo/redis"
	"github.com/zeebo/errs"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"regexp"
	"strings"
)

type redisDevice struct {
	//contract.DescribedObject this is what it was for 1.3 edgex foundry
	contract.DeviceResource
	Id             string
	Name           string
	AdminState     contract.AdminState
	OperatingState contract.OperatingState
	Protocols      map[string]contract.ProtocolProperties
	AutoEvents     []contract.AutoEvent
	Labels         []string
	Location       interface{}
	ServiceName    string
	ProfileName    string
}

func (dbClient *DeviceExtDBClient) GetFilterIdsByName(key string, name string) ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	dbClient.client.Logger.Infof("GetFilterIdsByName key: %s; name: %s", key, name)

	return dbClient.getFilterIdsByName(conn, key, []string{name})
}

func (dbClient *DeviceExtDBClient) getFilterIdsByName(conn redis.Conn, key string, names []string) ([]string, hedgeErrors.HedgeError) {
	var v []string
	for _, name := range names {
		// ZSCAN is a cursor based iterator, every call of the command the server returns an updated cursor that the user needs to use in the next call.
		// An iteration starts when the cursor is set to 0, and terminates when the cursor returned by the server is 0
		cursor := 0
		for {
			result, err := redis.Values(conn.Do("ZSCAN", key+name, cursor))
			if err != nil && !errs.Is(err, redis.ErrNil) {
				dbClient.client.Logger.Errorf("%v", err)
				return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error getting filter ids")
			}

			// Extract keys from the result
			keys := result[1].([]interface{})
			for index, key := range keys {
				if index%2 == 0 {
					v = append(v, string(key.([]byte)))
				}
			}

			// Update the cursor for the next iteration, in case cursor is back to 0 break
			cursor, _ = redis.Int(result[0], nil)
			if cursor == 0 {
				break
			}
		}
	}
	return v, nil
}

func (dbClient *DeviceExtDBClient) GetDeviceCount() int {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	return getCount(conn, db2.Device)
}

func getCount(conn redis.Conn, key string) int {
	count, err := redis.Int(conn.Do("ZCOUNT", key, "-inf", "+inf"))
	if err != nil {
		return 0
	}
	return count
}
func (dbClient *DeviceExtDBClient) GetDeviceByName(key string) ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Failed to get device by name: %s", key)

	//regex := "*"
	//for _, e := range key {
	//	if e != '-' && e != '_' && e != '~' {
	//		lower := strings.ToLower(string(e))
	//		upper := strings.ToUpper(string(e))
	//		regex += "[" + lower + upper + "]"
	//	} else {
	//		regex += "[" + string(e) + "]"
	//	}
	//}
	//regex += "*"
	//
	//ids, err := redis.Values(conn.Do("HSCAN", "md|dv:name", 1, "MATCH", regex))
	////ids, err := redis.Values(conn.Do("HSCAN", "md|dv:name", 1, "MATCH", "/"+key+"/i"))
	//if err != nil {
	//	dbClient.Logger.Errorf("%v", err)
	//}
	//keyInterfaces := ids[1].([]interface{})
	//// keys := make([]string, len(keyInterfaces))
	//var k []string
	//for index, keyInterface := range keyInterfaces {
	//	if index%2 != 0 {
	//		k = append(k, string(keyInterface.([]byte)))
	//	}
	//	// keys[index] = string(keyInterface.([]byte))
	//}
	//service := interfaces.ApplicationService
	var k []string
	keys, err := redis.Strings(conn.Do("HKEYS", "md|dv:name"))
	if err != nil {
		dbClient.client.Logger.Errorf("%v", err)

		return k, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	//ids := []string{}

	for _, val := range keys {
		if checkRegex(key, val) {
			id, err := redis.String(conn.Do("HGET", "md|dv:name", val))
			if err != nil {
				if errs.Is(err, redis.ErrNil) {
					dbClient.client.Logger.Debugf("%s: Field %s not found in the hash", errorMessage, val)
				} else {
					return k, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
				}

			}
			k = append(k, id)
		}
	}

	return k, nil
}

func checkRegex(key string, dvName string) bool {
	regex := ".*"
	for _, e := range key {
		if e != '-' && e != '_' && e != '~' {
			lower := strings.ToLower(string(e))
			upper := strings.ToUpper(string(e))
			regex += "[" + lower + upper + "]"
		} else {
			regex += "[" + string(e) + "]"
		}
	}
	regex += ".*"
	match, err := regexp.MatchString(regex, dvName)
	if err != nil {
		return false
	}
	return match
}

func (dbClient *DeviceExtDBClient) GetDevices(start int, end int) ([]dtos.Device, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	// Get count of devices

	// decide the start and end
	objects, err := redis2.GetObjectsByRange(conn, db2.Device, start, end)
	if err != nil {
		dbClient.client.Logger.Errorf("Error geting devices from %v to %v: %v", start, end, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error getting devices")
	}

	var unmarshalO []dtos.Device
	unmarshalO, err = unmarshalRedisDevice(conn, objects)
	if err != nil {
		dbClient.client.Logger.Errorf("Error geting devices from %v to %v: %v", start, end, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting devices")
	}

	return unmarshalO, nil
}

func (dbClient *DeviceExtDBClient) GetDevicesUsingIds(ids []string) ([]dtos.Device, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Failed to get device using ids: %v", ids)

	args := redis.Args{}
	for _, v := range ids {
		args = args.Add(v)
	}

	if len(args) == 0 {
		return make([]dtos.Device, 0), nil
	}

	objects, err := redis.ByteSlices(conn.Do("MGET", args...))
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var unmarshalO []dtos.Device
	unmarshalO, err = unmarshalRedisDevice(conn, objects)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return unmarshalO, nil
}

func (dbClient *DeviceExtDBClient) GetDeviceIdsUsingFilters(id []string) ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Failed to get device ids using filters: %v", id)

	args := redis.Args{}
	for _, v := range id {
		args = args.Add(v)
	}
	// ids, err := redis.Strings(conn.Do("SUNION", args...))
	ids, err := redis.Strings(conn.Do("SINTER", args...))
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return ids, nil
}

func unmarshalRedisDevice(conn redis.Conn, objects [][]byte) ([]dtos.Device, error) {
	var unmarshalObjects []dtos.Device
	var err error
	type UService struct{ Name string }
	//var unmarshalDeviceService UService

	for _, o := range objects {

		var s redisDevice
		var x dtos.Device
		json.Unmarshal(o, &s)

		x.Id = s.Id
		x.Name = s.Name
		x.AdminState = string(s.AdminState)
		x.OperatingState = string(s.OperatingState)
		x.Labels = s.Labels
		x.Location = s.Location
		x.ProfileName = s.ProfileName
		x.ServiceName = s.ServiceName

		unmarshalObjects = append(unmarshalObjects, x)
	}
	return unmarshalObjects, err
}
