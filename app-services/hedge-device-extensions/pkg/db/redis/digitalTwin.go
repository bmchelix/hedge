/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	db2 "hedge/common/db"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"strconv"
	"strings"
)

// Scene queries
func (dbClient *DeviceExtDBClient) GetDigitalTwin(sceneId string) (dto.Scene, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var dt dto.Scene

	scene, er := conn.Do("HGETALL", db2.DTwinScene+":"+sceneId)
	if len(scene.([]interface{})) == 0 {
		dbClient.client.Logger.Debug("Scene ID not found in db")
		return dt, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("SceneId %s not found", sceneId))
	}
	res, er := redis.StringMap(scene, er)
	resByte, er := json.Marshal(res)
	json.Unmarshal(resByte, &dt)
	json.Unmarshal([]byte(res["displayAttrib"]), &dt.DisplayAttrib)

	if er != nil {
		dbClient.client.Logger.Debugf("Error getting digital-twin from db: %s", er.Error())
		return dt, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting sceneId %s", sceneId))
	}
	return dt, nil
}

func (dbClient *DeviceExtDBClient) CreateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	exists, err := keyExist(conn, db2.DTwinScene+":"+scene.SceneId, scene.SceneId)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error from db %s", err.Error()))
	} else if exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Scene %s already exist ", scene.SceneId))
	}

	res, er := conn.Do("FT.SEARCH", db2.DTwinScene, "@deviceId:"+scene.DeviceId)
	if res.([]interface{})[0].(int64) > 0 {
		dbClient.client.Logger.Debug("deviceId already found in another scene")
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error finding device %s", scene.DeviceId))
	}

	attr, _ := json.Marshal(scene.DisplayAttrib)
	_, er = conn.Do("HMSET", db2.DTwinScene+":"+scene.SceneId, "sceneId", scene.SceneId, "imageId", scene.ImageId,
		"deviceId", scene.DeviceId, "sceneType", scene.SceneType, "displayAttrib", attr)
	if er != nil {
		dbClient.client.Logger.Errorf("Error while creating scene in db: ", er.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error creating scene %s", scene.SceneId))
	}
	return nil
}

func (dbClient *DeviceExtDBClient) UpdateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	//exists, err := keyExist(conn, db2.DTwinScene+":"+scene.SceneId, scene.SceneId)
	//if err != nil {
	//	return err
	//} else if !exists {
	//	return db2.ErrNotFound
	//}
	res, err := conn.Do("FT.SEARCH", db2.DTwinScene, "@sceneId:"+scene.SceneId+" @deviceId:"+scene.DeviceId)
	if res.([]interface{})[0].(int64) == 0 {
		dbClient.client.Logger.Debug("scene with the provided sceneId and deviceId not found")
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error finding scene %s with device %s", scene.SceneId, scene.DeviceId))
	}

	attr, _ := json.Marshal(scene.DisplayAttrib)
	_, err = conn.Do("HSET", db2.DTwinScene+":"+scene.SceneId, "sceneId", scene.SceneId, "imageId", scene.ImageId,
		"deviceId", scene.DeviceId, "sceneType", scene.SceneType, "displayAttrib", attr)
	if err != nil {
		dbClient.client.Logger.Errorf("Error while updating scene in db: ", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error updating scene %s with device %s", scene.SceneId, scene.DeviceId))
	}
	return nil
}

func (dbClient *DeviceExtDBClient) DeleteDigitalTwin(sceneId string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	exists, err := keyExist(conn, db2.DTwinScene+":"+sceneId, sceneId)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error from db %s", err.Error()))
	} else if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error finding scene %s", sceneId))
	}

	_, er := conn.Do("DEL", db2.DTwinScene+":"+sceneId)
	if err != nil {
		dbClient.client.Logger.Errorf("Error while deleting scene from db: %s", er.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error deleting scene %s", sceneId))
	}
	return nil
}

// Device queries
func (dbClient *DeviceExtDBClient) GetDeviceById(deviceId string) (dto.DeviceObject, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var device dto.DeviceObject

	scene, err := conn.Do("FT.SEARCH", db2.DTwinScene, "@deviceId:"+deviceId)
	if scene.([]interface{})[0].(int64) == 0 {
		dbClient.client.Logger.Debug("Scene not found in db")
		return device, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error finding device %s", deviceId))
	}
	res, err := redis.StringMap(scene.([]interface{})[2], err)
	resbyte, err := json.Marshal(res)
	json.Unmarshal(resbyte, &device)

	if err != nil {
		dbClient.client.Logger.Debugf("Error getting device by Id: %s", err.Error())
		return device, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", err.Error()))
	}
	return device, nil
}

func (dbClient *DeviceExtDBClient) GetDTwinByDevice(deviceId string) (dto.Scene, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var dt dto.Scene

	//replacer := strings.NewReplacer(" ", "%", "-", "%")
	scene, err := conn.Do("FT.SEARCH", db2.DTwinScene, "@deviceId:"+deviceId)
	if err != nil || scene.([]interface{})[0].(int64) == 0 {
		dbClient.client.Logger.Debugf("Scene not found in db (err:%s)", err)
		return dt, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error finding scene w/device %s", deviceId))
	}
	res, err := redis.StringMap(scene.([]interface{})[2], err)
	resbyte, err := json.Marshal(res)
	json.Unmarshal(resbyte, &dt)
	json.Unmarshal([]byte(res["displayAttrib"]), &dt.DisplayAttrib)

	if err != nil {
		dbClient.client.Logger.Debugf("Error getting digital-twin from db: %s", err.Error())
		return dt, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", err.Error()))
	}
	return dt, nil
}

// Image queries
func (dbClient *DeviceExtDBClient) SaveImageObject(image dto.Image) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	exists, err := keyExist(conn, db2.DTwinImg+":"+image.ImgId, image.ImgId)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error from db %s", err.Error()))
	} else if exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Image exist %s", image.ImgId))
	}

	_, er := conn.Do("HSET", db2.DTwinImg+":"+image.ImgId, "imgId", image.ImgId,
		"object", image.Object, "objName", image.ObjName, "remoteFileId", image.RemoteFileId)
	if err != nil {
		dbClient.client.Logger.Errorf("Error saving image in db: ", er.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error from db %s", er.Error()))
	}
	return nil
}

func (dbClient *DeviceExtDBClient) GetImageObject(image dto.Image) (dto.Image, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var img dto.Image
	var err hedgeErrors.HedgeError

	if image.ImgId != "" {
		err = getImage(conn, &img, "imgId", image.ImgId) // image by id
	} else if image.Object != "" {
		err = getImage(conn, &img, "objName", image.Object) // image by device id
		if img.ImgId == "" && image.ObjName != "" {
			err = getImage(conn, &img, "objName", image.ObjName) // image by profile id
		}
	} else if image.ObjName != "" && img.ImgId == "" {
		err = getImage(conn, &img, "objName", image.ObjName) // image by profile id
	}
	if err != nil {
		dbClient.client.Logger.Errorf("Error getting scene from db: %s", err.Error())
		return img, err
	}
	return img, nil
}

func (dbClient *DeviceExtDBClient) GetImageIds() ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var ids []string
	var cursor int

	for {
		result, err := conn.Do("SCAN", cursor, "MATCH", db2.DTwinImg+":*", "COUNT", 100)
		if err != nil {
			dbClient.client.Logger.Errorf("Error image keys from db: %s", err.Error())
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "failed to get keys from db")
		}

		cursor, err = strconv.Atoi(string(result.([]interface{})[0].([]uint8)))
		if err != nil {
			dbClient.client.Logger.Errorf("error parsing cursor image keys from db: %s", err.Error())
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "failed to parse cursor from db")
		}
		sceneKeys := result.([]interface{})[1].([]interface{})

		for _, key := range sceneKeys {
			ids = append(ids, strings.TrimPrefix(string(key.([]uint8)), db2.DTwinImg+":"))
		}

		if cursor == 0 {
			break
		}
	}
	return ids, nil
}

func (dbClient *DeviceExtDBClient) GetSceneIds() ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var ids []string
	var cursor int

	for {
		result, err := conn.Do("SCAN", cursor, "MATCH", db2.DTwinScene+":*", "COUNT", 100)
		if err != nil {
			dbClient.client.Logger.Errorf("error getting scene keys from db: %s", err.Error())
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "failed to get keys from db")
		}

		cursor, err = strconv.Atoi(string(result.([]interface{})[0].([]uint8)))
		if err != nil {
			dbClient.client.Logger.Errorf("error parsing cursor scene keys from db: %s", err.Error())
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "failed to parse cursor from db")
		}
		imageKeys := result.([]interface{})[1].([]interface{})

		for _, key := range imageKeys {
			ids = append(ids, strings.TrimPrefix(string(key.([]uint8)), db2.DTwinScene+":"))
		}

		if cursor == 0 {
			break
		}
	}
	return ids, nil
}

func (dbClient *DeviceExtDBClient) GetAllImages(image dto.Image) (dto.Image, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	var img dto.Image

	err := getImage(conn, &img, "imgId", image.ImgId)
	if err != nil {
		dbClient.client.Logger.Errorf("Error while getting all images from db: %s", err.Error())
	}
	return img, err
}

func (dbClient *DeviceExtDBClient) DeleteImageObject(imageId string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	exists, err := keyExist(conn, db2.DTwinImg+":"+imageId, imageId)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error from db %s", err.Error()))
	} else if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Image %s not found", imageId))
	}

	_, er := conn.Do("DEL", db2.DTwinImg+":"+imageId)
	if er != nil {
		dbClient.client.Logger.Errorf("Error while deleting scene from db: %s", er.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error deleting image %s", imageId))
	}
	return nil
}

// Helper funcs
func (dbClient *DeviceExtDBClient) CreateIndex() hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("FT.CREATE", db2.DTwinScene, "ON", "HASH", "PREFIX", "1", db2.DTwinScene+":",
		"SCHEMA", "sceneId", "TEXT", "SORTABLE", "imageId", "TEXT", "SORTABLE", "deviceId", "TEXT", "SORTABLE")
	if err != nil && err.Error() != "Index already exists" {
		dbClient.client.Logger.Errorf("Error while creating INDEX: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error creating scene index"))
	}
	_, err = conn.Do("FT.CREATE", db2.DTwinImg, "ON", "HASH", "PREFIX", "1", db2.DTwinImg+":",
		"SCHEMA", "imageId", "TEXT", "SORTABLE", "imgId", "TEXT", "SORTABLE", "object", "TEXT", "SORTABLE",
		"objName", "TEXT", "SORTABLE")
	if err != nil && err.Error() != "Index already exists" {
		dbClient.client.Logger.Errorf("Error while creating INDEX: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error creating image index"))
	}
	return nil
}

func keyExist(conn redis.Conn, key string, val string) (bool, error) {
	exists, err := redis.Bool(conn.Do("EXISTS", key, val))
	return exists, err
}

func getImage(conn redis.Conn, img *dto.Image, key string, id string) hedgeErrors.HedgeError {
	//var replacer *strings.Replacer
	//if strings.ToUpper(filepath.Ext(id)) == ".GLB" {
	//	replacer = strings.NewReplacer(" ", "%", "-", "%", ".", "%")
	//} else {
	//	replacer = strings.NewReplacer(" ", "%", "-", "%")
	//}

	image, err := conn.Do("FT.SEARCH", db2.DTwinImg, "@"+key+":"+id)
	if image.([]interface{})[0].(int64) == 0 {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error finding image %s", id))
	}
	res, err := redis.StringMap(image.([]interface{})[2], err)
	resbyte, err := json.Marshal(res)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", err.Error()))
	}
	json.Unmarshal(resbyte, &img)
	return nil
}
