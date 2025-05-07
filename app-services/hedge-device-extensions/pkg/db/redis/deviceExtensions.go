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
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	dto2 "hedge/common/dto"
	hedgeErrors "hedge/common/errors"
)

func (dbClient *DeviceExtDBClient) AddDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	uid, err := dbClient.addProfileExt(conn, profileName, deviceExtns)
	if err != nil {
		dbClient.client.Logger.Errorf("Error adding device extensions in profile %s: %s", profileName, err.Error())
		return "", err
	}

	return uid, nil
}

func (dbClient *DeviceExtDBClient) addProfileExt(conn redis.Conn, profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError) {
	dexists, err := redis.Bool(conn.Do("HEXISTS", db2.DeviceProfile+":name", profileName)) //profile exists

	if err != nil {
		dbClient.client.Logger.Errorf("Error adding device extensions in profile %s: %s", profileName, err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error getting profile %s", profileName))
	}
	if !dexists {
		dbClient.client.Logger.Errorf(fmt.Sprintf("Profile %s not found", profileName))
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Profile %s not found", profileName))
	}

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.ProfileExt+":name", profileName)) //profile extension exists
	if err != nil {
		dbClient.client.Logger.Errorf("Error adding device extension in profile %s: %v", profileName, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error getting profile extension for profile %s", profileName))
	}
	if exists {
		dbClient.client.Logger.Errorf("Extension for profile %s already exists", profileName)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Extension for profile %s already exists", profileName))
	}

	m, err := marshalObject(deviceExtns)
	attributes := make([]string, 0)
	uid := uuid.New().String()
	id := db2.ProfileExt + ":" + uid
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", id, m)                                    // Store payload against ID
	_ = conn.Send("HSET", db2.ProfileExt+":name", profileName, id) // Map the profile name with extension id
	for _, profileExt := range deviceExtns {
		if profileExt.Field == "" {
			//Don't entertain blank fields
			continue
		}
		attributes = append(attributes, profileExt.Field)
		var isMandatory = "0"
		if profileExt.IsMandatory {
			isMandatory = "1"
		}
		//Map the extended field to "value|<0 or 1>" 0=optional, 1=mandatory Eg. serialNo = 12345|0
		_ = conn.Send("HSET", db2.ProfileExt+":"+profileName, profileExt.Field, profileExt.Default+"|"+isMandatory)
	}
	//_ = conn.Send("HSET", db2.ProfileExt+":"+profileName+":devices", nil, nil)

	// Add attribute to profile mapping
	dbClient.addAttributeToProfileLookup(conn, attributes, profileName)

	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("Error Saving Association in DataBase: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error Saving Association: profile %s", profileName))
	}
	return uid, nil
}

func (dbClient *DeviceExtDBClient) addAttributeToProfileLookup(conn redis.Conn, attributes []string, profileName string) hedgeErrors.HedgeError {
	//
	var err error
	for _, attr := range attributes {
		err = conn.Send("SADD", db2.ProfileAttr+":"+attr, profileName)
	}

	if err != nil {
		dbClient.client.Logger.Errorf("Error adding attribute to profile %s: %v", profileName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error adding attribute to profile %s", profileName))
	}

	return nil
}

// Part of larger EXEC send command, not an independent one
func (dbClient *DeviceExtDBClient) removeProfileFromAttributesLookup(conn redis.Conn, attributes []string, profileName string) hedgeErrors.HedgeError {
	//
	var err error
	for _, attr := range attributes {
		err = conn.Send("SREM", db2.ProfileAttr+":"+attr, profileName)
	}

	if err != nil {
		dbClient.client.Logger.Errorf("Error removing attribute from profile %s: %v", profileName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error removing attribute from profile %s", profileName))
	}

	return nil
}

// GetProfilesByAttribute Gets the list of profiles for the given attribute list
func (dbClient *DeviceExtDBClient) GetProfilesByAttribute(attributes []string) ([]string, hedgeErrors.HedgeError) {
	if attributes == nil || len(attributes) == 0 {
		dbClient.client.Logger.Warn("Attributes are empty")
		return make([]string, 0), nil
	}
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	unionKeys := make([]interface{}, len(attributes))
	for i, attr := range attributes {
		unionKeys[i] = db2.ProfileAttr + ":" + attr
	}

	profiles, err := redis.Strings(conn.Do("SUNION", unionKeys...))
	if err != nil {
		dbClient.client.Logger.Errorf("Error getting profile by attributes: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Error getting profile by attributes")
	}

	return profiles, nil
}

func (dbClient *DeviceExtDBClient) UpdateDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting device extensions for profile %s", profileName)

	dexists, err := redis.Bool(conn.Do("HEXISTS", db2.DeviceProfile+":name", profileName)) //profile exists
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !dexists {
		dbClient.client.Logger.Errorf("%s: Device profile not found", errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device profile not found", errorMessage))
	}

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.ProfileExt+":name", profileName)) //device profile extension exists
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !exists {
		dbClient.client.Logger.Errorf("%s: Profile extension not found", errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile extension not found", errorMessage))
	}

	err = dbClient.deleteProfileExt(conn, profileName, false)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	return dbClient.addProfileExt(conn, profileName, deviceExtns)
}

func (dbClient *DeviceExtDBClient) GetDeviceExtensionInProfile(profileName string) ([]dto2.DeviceExtension, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var deviceExtns []dto2.DeviceExtension
	err := redis2.GetObjectByKey(conn, db2.ProfileExt+":name", profileName, &deviceExtns)
	if err != nil {
		if errors.Is(err, db2.ErrNotFound) {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Device Profile Extension: profile %s: Profile Extension not found", profileName))
		}
		dbClient.client.Logger.Errorf("Error Getting Device Profile Extension from DataBase: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error Getting Device Profile Extension: profile %s", profileName))
	}

	return deviceExtns, nil
}

func (dbClient *DeviceExtDBClient) DeleteDeviceExtensionInProfile(profileName string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	err := dbClient.deleteProfileExt(conn, profileName, true)
	return err
}

func (dbClient *DeviceExtDBClient) deleteProfileExt(conn redis.Conn, profileName string, hardDelete bool) hedgeErrors.HedgeError {
	pexists, err := redis.Bool(conn.Do("HEXISTS", db2.DeviceProfile+":name", profileName)) //profile exists

	errorMessage := fmt.Sprintf("Error Deleting Profile extensions for profile %s", profileName)

	if err != nil {
		dbClient.client.Logger.Errorf("Error retrieving value from DB for key %s:%s", db2.DeviceProfile, profileName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)

	}
	if !pexists {
		dbClient.client.Logger.Errorf("Device profile not found: %s", profileName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound,
			fmt.Sprintf("Device profile not found: %s", profileName))
	}

	id, err1 := redis.String(conn.Do("HGET", db2.ProfileExt+":name", profileName))
	if err1 != nil {
		dbClient.client.Logger.Errorf("Extension not found for profile: %s: %v", profileName, err1)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Extension not found for profile: %s", profileName))
	}

	var deviceExtns []dto2.DeviceExtension
	attributes := make([]string, 0)
	err = redis2.GetObjectByKey(conn, db2.ProfileExt+":name", profileName, &deviceExtns)
	if err == nil {
		for _, devProfileExt := range deviceExtns {
			attributes = append(attributes, devProfileExt.Field)
		}
	}

	if hardDelete {
		//if device extensions are associated with this profile, then do not delete
		smems, err := redis.Strings(conn.Do("SMEMBERS", db2.ProfileDevicesExt+":"+profileName))
		if err != nil {
			dbClient.client.Logger.Errorf(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error retrieving device extensions for profile %s", profileName))
		}
		if len(smems) > 0 {
			errStr := fmt.Sprintf("Fail to delete profile attribute(s) for %s when associated device exists: %v", profileName, strings.Join(smems, ", "))
			dbClient.client.Logger.Errorf(errStr)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, errStr)
		}
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", id)
	_ = conn.Send("DEL", db2.ProfileExt+":"+profileName) //delete complete hash of field-vals
	_ = conn.Send("HDEL", db2.ProfileExt+":name", profileName)

	dbClient.removeProfileFromAttributesLookup(conn, attributes, profileName)

	//For soft deletes do not delete profileExt to devices mapping
	if hardDelete {
		_ = conn.Send("DEL", db2.ProfileDevicesExt+":"+profileName)
	}
	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting Device Profile extension from DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error deleting Device Profile extension for profileName %s", profileName))
	}

	return nil
}

func (dbClient *DeviceExtDBClient) AddDeviceExt(deviceName string, profileName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	return dbClient.addDeviceExt(conn, deviceName, profileName, deviceExts)
}

func (dbClient *DeviceExtDBClient) addDeviceExt(conn redis.Conn, deviceName string, profileName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError) {
	dexists, err := redis.Bool(conn.Do("HEXISTS", db2.Device+":name", deviceName)) //device exists
	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error adding device extensions for profile %s", profileName))
	}
	if !dexists {
		dbClient.client.Logger.Errorf("Device extensions for profile %s not found", profileName)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Device extensions for profile %s not found", profileName))
	}

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.DeviceExt+":name", deviceName)) //device extension exists
	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error adding device extensions for profile %s", profileName))
	}
	if exists {
		dbClient.client.Logger.Errorf("Device extension for profile %s already existss", profileName)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Device extension for profile %s already existss", profileName))
	}

	m, err := marshalObject(deviceExts)
	uid := uuid.New().String()
	id := db2.DeviceExt + ":" + uid
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", id, m)                                  // Store payload against ID
	_ = conn.Send("HSET", db2.DeviceExt+":name", deviceName, id) // Map the device name with extension id
	for _, deviceExt := range deviceExts {
		_ = conn.Send("HSET", db2.DeviceExt+":"+deviceName, deviceExt.Field, deviceExt.Value)
	}
	_ = conn.Send("SADD", db2.ProfileDevicesExt+":"+profileName, deviceName)
	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Debugf("Error Saving Association in DataBase: %s", err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Failed to save device extensions for profile %s and device %s", profileName, deviceName))
	}

	return uid, nil
}

func (dbClient *DeviceExtDBClient) UpdateDeviceExt(deviceName string, profileName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error updating device extensions for profile %s and device %s", profileName, deviceName)

	dexists, err := redis.Bool(conn.Do("HEXISTS", db2.Device+":name", deviceName)) //device exists
	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !dexists {
		dbClient.client.Logger.Errorf("Device %s not found", deviceName)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device not found", errorMessage))
	}

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.DeviceExt+":name", deviceName)) //device extension exists
	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !exists {
		dbClient.client.Logger.Errorf("Device extensions for profile %s not found", profileName)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device extensions not found", errorMessage))
	}

	err = dbClient.deleteAllDeviceExt(conn, deviceName, profileName)
	if err != nil {
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return dbClient.addDeviceExt(conn, deviceName, profileName, deviceExts)
}

func (dbClient *DeviceExtDBClient) GetDeviceExt(deviceName string) ([]dto2.DeviceExt, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var deviceExt = make([]dto2.DeviceExt, 0)
	err := redis2.GetObjectByKey(conn, db2.DeviceExt+":name", deviceName, &deviceExt)
	if err != nil {
		if errors.Is(err, db2.ErrNotFound) {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error Getting Device Extension: device %s: Device Extension not found", deviceName))
		} else {
			dbClient.client.Logger.Errorf("Error Getting Device Extension from DataBase: %v", err)
		}
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error Getting Device Extension: device %s", deviceName))
	}
	return deviceExt, nil
}

func (dbClient *DeviceExtDBClient) DeleteAllDeviceExt(deviceName string, profileName string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	err := dbClient.deleteAllDeviceExt(conn, deviceName, profileName)
	return err
}

func (dbClient *DeviceExtDBClient) DeleteDeviceExt(deviceName string, profileName string, attToDelete []string, updatedDeviceExt []dto2.DeviceExt) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	err := dbClient.deleteDeviceExt(conn, deviceName, profileName, attToDelete, updatedDeviceExt)
	return err
}

func (dbClient *DeviceExtDBClient) deleteAllDeviceExt(conn redis.Conn, deviceName string, profileName string) hedgeErrors.HedgeError {
	pexists, err := redis.Bool(conn.Do("HEXISTS", db2.Device+":name", deviceName)) //device exists

	errorMessage := fmt.Sprintf("Error deleting device extension device: %s profile %s", deviceName, profileName)

	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else if !pexists {
		dbClient.client.Logger.Errorf("%s: Device %s not found", errorMessage, deviceName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device %s not found", errorMessage, deviceName))
	}

	id, err := redis.String(conn.Do("HGET", db2.DeviceExt+":name", deviceName))
	if err != nil {
		dbClient.client.Logger.Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Extension not found for device: %s", errorMessage, deviceName))
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", id)
	_ = conn.Send("DEL", db2.DeviceExt+":"+deviceName) //delete complete hash of field-vals
	_ = conn.Send("HDEL", db2.DeviceExt+":name", deviceName)
	_ = conn.Send("SREM", db2.ProfileDevicesExt+":"+profileName, deviceName)
	_, err = conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting Device extension from DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) deleteDeviceExt(conn redis.Conn, deviceName string, profileName string, attToDelete []string, updatedDeviceExt []dto2.DeviceExt) hedgeErrors.HedgeError {
	pexists, err := redis.Bool(conn.Do("HEXISTS", db2.Device+":name", deviceName)) //device exists

	errorMessage := fmt.Sprintf("Error deleting device attributes for device: %s profile %s", deviceName, profileName)

	if err != nil {
		dbClient.client.Logger.Errorf(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	if !pexists {
		dbClient.client.Logger.Errorf("%s: Device %s not found", errorMessage, deviceName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device not found", errorMessage))
	}

	id, err := redis.String(conn.Do("HGET", db2.DeviceExt+":name", deviceName))

	if err != nil {
		dbClient.client.Logger.Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Attribute not found for device: %s", errorMessage, deviceName))
	}

	m, err := marshalObject(updatedDeviceExt)
	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", id)
	_ = conn.Send("SET", id, m)
	for _, attribute := range attToDelete {
		_ = conn.Send("HDEL", db2.DeviceExt+":"+deviceName, attribute)
	}
	_ = conn.Send("SREM", db2.ProfileDevicesExt+":"+profileName, deviceName)
	_, err = conn.Do("EXEC")
	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting Device attribute from DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	deviceAttributes, err := redis.Strings(conn.Do("HGETALL", db2.DeviceExt+":"+deviceName))
	if len(deviceAttributes) == 0 {
		_ = conn.Send("MULTI")
		_ = conn.Send("DEL", id)
		_ = conn.Send("HDEL", db2.DeviceExt+":name", deviceName)
		_, err = conn.Do("EXEC")
		if err != nil {
			dbClient.client.Logger.Errorf("Error deleting Device attribute from DataBase: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
	}

	return nil
}

func (dbClient *DeviceExtDBClient) UpsertProfileContextualAttributes(profileName string, contextualAttributes []string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error Upserting ProfileContextualAttributes for profile %s", profileName)

	m, err := marshalObject(contextualAttributes)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", db2.ProfileContextualData+":"+profileName, m)
	_ = dbClient.addAttributeToProfileLookup(conn, contextualAttributes, profileName)
	_, err = conn.Do("EXEC")
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) GetProfileContextualAttributes(profileName string) ([]string, hedgeErrors.HedgeError) {
	dbClient.client.Logger.Debugf("Getting ProfileContextualAttributes for profile %s", profileName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting Contextual Attributes for profile: %s", profileName)

	data, err := redis.Bytes(conn.Do("GET", db2.ProfileContextualData+":"+profileName))
	if err != nil {
		// Not found is ok
		if errors.Is(err, redis.ErrNil) {
			dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
			// Return empty array
			return make([]string, 0), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Could not find contextual attributes for profile: %s", profileName))
		}
		dbClient.client.Logger.Errorf("Error Getting Profile Contextual Attributes from DataBase for profileName %s: %v", profileName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var contextualAttrs []string
	err = unmarshalObject(data, &contextualAttrs)
	if err != nil {
		dbClient.client.Logger.Errorf("Error unmarshalling response from DataBase for profileName %s: %v", profileName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	return contextualAttrs, nil
}

func (dbClient *DeviceExtDBClient) DeleteProfileContextualAttributes(profileName string) hedgeErrors.HedgeError {
	dbClient.client.Logger.Debugf("Deleting ProfileContextualAttributes for profile %s", profileName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error deleting ProfileContextualAttributes for profile %s", profileName)

	attributes, hedgeErr := dbClient.GetProfileContextualAttributes(profileName)
	if hedgeErr != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, hedgeErr)
		return hedgeErr
	}

	_ = conn.Send("MULTI")
	_ = dbClient.removeProfileFromAttributesLookup(conn, attributes, profileName)
	_ = conn.Send("DEL", db2.ProfileContextualData+":"+profileName)
	_, err := conn.Do("EXEC")

	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) UpsertDeviceContextualAttributes(deviceName string, data map[string]interface{}) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error upserting DeviceContextualAttributes for device %s", deviceName)

	m, err := marshalObject(data)
	if err != nil {
		dbClient.client.Logger.Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	err = conn.Send("SET", db2.DeviceContextualData+":"+deviceName, m)
	if err != nil {
		dbClient.client.Logger.Error(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) GetDeviceContextualAttributes(deviceName string) (map[string]interface{}, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting DeviceContextualAttributes for device %s", deviceName)

	bytes, err := redis.Bytes(conn.Do("GET", db2.DeviceContextualData+":"+deviceName))
	if err != nil {
		// Not found is ok
		if errors.Is(err, redis.ErrNil) {
			// Return empty array
			dbClient.client.Logger.Debugf("%s: Not Found", errorMessage)
			return make(map[string]interface{}), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Not Found", errorMessage))
		}
		dbClient.client.Logger.Errorf("Error Getting Device Extension from DataBase: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var contextualData map[string]interface{}
	err = unmarshalObject(bytes, &contextualData)
	if err != nil {
		dbClient.client.Logger.Error(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return contextualData, nil
}

func (dbClient *DeviceExtDBClient) DeleteDeviceContextualAttributes(deviceName string) hedgeErrors.HedgeError {
	dbClient.client.Logger.Debugf("Deleting DeviceContextualAttributes for device %s", deviceName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	if err := conn.Send("DEL", db2.DeviceContextualData+":"+deviceName); err != nil {
		dbClient.client.Logger.Debugf("Error deleting DeviceContextualAttributes for device %s: %v", deviceName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "Failed to delete contextual attributes for device "+deviceName)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) GetProtocolsForService(svcName string) (map[string]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error getting Protocols for Service %s", svcName)

	protoMap, err := redis.StringMap(conn.Do("HGETALL", db2.DeviceServiceExt+":"+svcName))
	if err != nil {
		// Not found is ok
		if errors.Is(err, redis.ErrNil) {
			// Return empty array
			dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
			return make(map[string]string), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage)
		}
		dbClient.client.Logger.Errorf("Error Getting Protocols for the service %s from DataBase: %v", svcName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return protoMap, nil
}

func (dbClient *DeviceExtDBClient) UpsertDownsamplingConfig(profileName string, downsamplingConfig *dto2.DownsamplingConfig) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error updating downsampling config for profile %s", profileName)

	m, err := marshalObject(downsamplingConfig)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: error marshalling downsamplingConfig -> %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	err = conn.Send("SET", db2.ProfileDownsamplingConfig+":"+profileName, m)

	if err != nil {
		dbClient.client.Logger.Errorf("%s: redis error -> %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}
func (dbClient *DeviceExtDBClient) GetDownsamplingConfig(profileName string) (*dto2.DownsamplingConfig, hedgeErrors.HedgeError) {

	var downsamplingConfig *dto2.DownsamplingConfig
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	key := db2.ProfileDownsamplingConfig + ":" + profileName
	dbClient.client.Logger.Infof("Getting downsampling config for profile, key: %s", key)

	errorMessage := fmt.Sprintf("Error Getting Downsampling Config for profile %s", profileName)
	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			dbClient.client.Logger.Errorf("%s: ProfileDownsamplingConfig Not Found", errorMessage)
			return nil, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("%s: ProfileDownsamplingConfig Not Found", errorMessage),
			)
		}
		dbClient.client.Logger.Errorf("Error getting DownsamplingConfig from DataBase: %s", err.Error())
		return downsamplingConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	err = unmarshalObject(data, &downsamplingConfig)
	if err != nil {
		dbClient.client.Logger.Errorf("%s: %v", errorMessage, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	return downsamplingConfig, nil
}

func (dbClient *DeviceExtDBClient) DeleteDownsamplingConfig(profileName string) hedgeErrors.HedgeError {
	dbClient.client.Logger.Debugf("Deleting downsampling config for profile %s", profileName)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error Deleting Downsampling Config for profile %s", profileName)

	exists, err := redis.Bool(conn.Do("EXISTS", db2.ProfileDownsamplingConfig+":"+profileName)) //profile downsampling config exists
	if err != nil {
		dbClient.client.Logger.Errorf("%s from DataBase: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else if !exists {
		dbClient.client.Logger.Errorf("%s from DataBase: Profile downsampling config not found", errorMessage)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("%s: Profile downsampling config Not Found", errorMessage))
	}

	_, err = conn.Do("DEL", db2.ProfileDownsamplingConfig+":"+profileName)
	if err != nil {
		dbClient.client.Logger.Errorf("%s from DataBase: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) UpsertNodeRawDataConfigs(nodeRawDataConfigs []dto2.NodeRawDataConfig, existingConfigsToReplace []dto2.NodeRawDataConfig) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := "Error Updating raw data config"

	// Start the transaction for DEL operations
	_ = conn.Send("MULTI")
	for _, confToDelete := range existingConfigsToReplace {
		err := dbClient.deleteNodeRawDataConfig(conn, confToDelete.Node.NodeID)
		if err != nil {
			dbClient.client.Logger.Errorf("Error deleting existing raw data config from DataBase: %v", err)
			return err
		}
	}
	// Execute the DEL transaction
	_, err := conn.Do("EXEC")
	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting existing raw data config from DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	// Start the transaction for SET operations
	_ = conn.Send("MULTI")
	for _, conf := range nodeRawDataConfigs {
		err := dbClient.addNodeRawDataConfig(conn, conf.Node.NodeID, conf)
		if err != nil {
			dbClient.client.Logger.Errorf("Error upserting raw data config in DataBase: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
		}
	}
	// Execute the SET transaction
	_, err = conn.Do("EXEC")
	if err != nil {
		dbClient.client.Logger.Errorf("Error upserting raw data config in DataBase: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) addNodeRawDataConfig(conn redis.Conn, nodeID string, nodeRawDataConfig dto2.NodeRawDataConfig) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error adding raw data config for node %s", nodeID)

	m, err := marshalObject(nodeRawDataConfig)
	if err != nil {
		dbClient.client.Logger.Errorf("Error saving raw data config in DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	// Queue the SET command in the transaction
	err = conn.Send("SET", db2.NodeRawDataConfig+":"+nodeID, m)
	if err != nil {
		dbClient.client.Logger.Errorf("Error saving raw data config in DataBase: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (dbClient *DeviceExtDBClient) deleteNodeRawDataConfig(conn redis.Conn, nodeID string) hedgeErrors.HedgeError {
	id, err := redis.String(conn.Do("HGET", db2.NodeRawDataConfig+":name", nodeID))
	if err != nil {
		dbClient.client.Logger.Errorf("Error deleting raw data config from node %s: %v", nodeID, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Raw data config not found for node: %s", nodeID))
	}

	// Queue the DEL command in the transaction
	if err = conn.Send("DEL", id); err != nil {
		dbClient.client.Logger.Errorf("Error deleting raw data config from node %s: %v", nodeID, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error deleting raw data config for node: %s", nodeID))
	}
	return nil
}

func (dbClient *DeviceExtDBClient) GetNodeRawDataConfigByNodeID(nodeId string) (*dto2.NodeRawDataConfig, hedgeErrors.HedgeError) {
	dbClient.client.Logger.Debugf("Getting raw data config by nodeId %s", nodeId)

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error Getting raw data config by NodeID: %s", nodeId)

	var nodeRawDataConfig *dto2.NodeRawDataConfig
	data, err := redis.Bytes(conn.Do("GET", db2.NodeRawDataConfig+":"+nodeId))
	if err != nil {
		dbClient.client.Logger.Errorf("Error getting raw data config from DataBase: %v", err)

		if errors.Is(err, redis.ErrNil) {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("raw data config not found for node: %s", nodeId))
		}

		return nodeRawDataConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	err = unmarshalObject(data, &nodeRawDataConfig)
	if err != nil {
		dbClient.client.Logger.Errorf("Error getting raw data config from DataBase: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	return nodeRawDataConfig, nil
}
