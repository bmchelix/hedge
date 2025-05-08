/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/go-redsync/redsync/v4"
	"hedge/common/db"
	"hedge/common/db/redis"
	dto2 "hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
)

type DeviceExtDBClient struct {
	client *redis.DBClient
}

var DBClientImpl DeviceExtDBClientInterface

type DeviceExtDBClientInterface interface {
	redis.CommonRedisDBInterface
	GetDbClient(dbConfig *db.DatabaseConfig, logger logger.LoggingClient) DeviceExtDBClientInterface
	AddAssociation(nodeA string, nodeAType string, associationNodes []dto2.AssociationNode) (string, hedgeErrors.HedgeError)
	GetAssociationByNodeName(node string) ([]dto2.AssociationNode, hedgeErrors.HedgeError)
	UpdateAssociation(nodeA string, nodeAType string, associationNode []dto2.AssociationNode) (string, hedgeErrors.HedgeError)
	DeleteAssociationByNodeName(node string) hedgeErrors.HedgeError
	AddLocation(location models.Location) (string, hedgeErrors.HedgeError)
	GetLocationById(id string) (models.Location, hedgeErrors.HedgeError)
	GetLocations(key string) ([]models.Location, hedgeErrors.HedgeError)
	AddDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError)
	GetProfilesByAttribute(attributes []string) ([]string, hedgeErrors.HedgeError)
	UpdateDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError)
	GetDeviceExtensionInProfile(profileName string) ([]dto2.DeviceExtension, hedgeErrors.HedgeError)
	DeleteDeviceExtensionInProfile(profileName string) hedgeErrors.HedgeError
	AddDeviceExt(deviceName string, profileName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError)
	UpdateDeviceExt(deviceName string, profileName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError)
	GetDeviceExt(deviceName string) ([]dto2.DeviceExt, hedgeErrors.HedgeError)
	DeleteAllDeviceExt(deviceName string, profileName string) hedgeErrors.HedgeError
	DeleteDeviceExt(deviceName string, profileName string, attToDelete []string, updatedDeviceExt []dto2.DeviceExt) hedgeErrors.HedgeError
	UpsertProfileContextualAttributes(profileName string, bizAttributes []string) hedgeErrors.HedgeError
	GetProfileContextualAttributes(profileName string) ([]string, hedgeErrors.HedgeError)
	DeleteProfileContextualAttributes(profileName string) hedgeErrors.HedgeError
	UpsertDeviceContextualAttributes(deviceName string, data map[string]interface{}) hedgeErrors.HedgeError
	GetDeviceContextualAttributes(deviceName string) (map[string]interface{}, hedgeErrors.HedgeError)
	DeleteDeviceContextualAttributes(deviceName string) hedgeErrors.HedgeError
	GetProtocolsForService(svcName string) (map[string]string, hedgeErrors.HedgeError)
	GetFilterIdsByName(key string, names string) ([]string, hedgeErrors.HedgeError)
	GetDeviceCount() int
	GetDeviceByName(key string) ([]string, hedgeErrors.HedgeError)
	GetDevices(start int, end int) ([]dtos.Device, hedgeErrors.HedgeError)
	GetDevicesUsingIds(ids []string) ([]dtos.Device, hedgeErrors.HedgeError)
	GetDeviceIdsUsingFilters(id []string) ([]string, hedgeErrors.HedgeError)
	UpsertDownsamplingConfig(profileName string, downsamplingConfig *dto2.DownsamplingConfig) hedgeErrors.HedgeError
	GetDownsamplingConfig(profileName string) (*dto2.DownsamplingConfig, hedgeErrors.HedgeError)
	DeleteDownsamplingConfig(profileName string) hedgeErrors.HedgeError

	UpsertNodeRawDataConfigs(nodeRawDataConfigs []dto2.NodeRawDataConfig, existingConfigsToReplace []dto2.NodeRawDataConfig) hedgeErrors.HedgeError
	GetNodeRawDataConfigByNodeID(nodeId string) (*dto2.NodeRawDataConfig, hedgeErrors.HedgeError)

	GetDigitalTwin(sceneId string) (dto2.Scene, hedgeErrors.HedgeError)
	CreateDigitalTwin(scene dto2.Scene) hedgeErrors.HedgeError
	UpdateDigitalTwin(scene dto2.Scene) hedgeErrors.HedgeError
	DeleteDigitalTwin(sceneId string) hedgeErrors.HedgeError
	GetDeviceById(deviceId string) (dto2.DeviceObject, hedgeErrors.HedgeError)
	GetDTwinByDevice(deviceId string) (dto2.Scene, hedgeErrors.HedgeError)
	SaveImageObject(image dto2.Image) hedgeErrors.HedgeError
	GetImageObject(image dto2.Image) (dto2.Image, hedgeErrors.HedgeError)
	GetAllImages(image dto2.Image) (dto2.Image, hedgeErrors.HedgeError)
	DeleteImageObject(imageId string) hedgeErrors.HedgeError
	GetImageIds() ([]string, hedgeErrors.HedgeError)
	GetSceneIds() ([]string, hedgeErrors.HedgeError)
	CreateIndex() hedgeErrors.HedgeError
}

func init() {
	DBClientImpl = &DeviceExtDBClient{}
}

func (dbClient *DeviceExtDBClient) GetDbClient(dbConfig *db.DatabaseConfig, logger logger.LoggingClient) DeviceExtDBClientInterface {
	dbc := redis.CreateDBClient(dbConfig)
	dbc.Logger = logger
	return &DeviceExtDBClient{client: dbc}
}

func NewDBClient(dbConfig *db.DatabaseConfig, logger logger.LoggingClient) DeviceExtDBClientInterface {
	return DBClientImpl.GetDbClient(dbConfig, logger)
}

func (dbClient *DeviceExtDBClient) IncrMetricCounterBy(key string, value int64) (int64, hedgeErrors.HedgeError) {
	return dbClient.client.IncrMetricCounterBy(key, value)
}

func (dbClient *DeviceExtDBClient) SetMetricCounter(key string, value int64) hedgeErrors.HedgeError {
	return dbClient.client.SetMetricCounter(key, value)
}

func (dbClient *DeviceExtDBClient) GetMetricCounter(key string) (int64, hedgeErrors.HedgeError) {
	return dbClient.client.GetMetricCounter(key)
}

func (dbClient *DeviceExtDBClient) AcquireRedisLock(name string) (*redsync.Mutex, hedgeErrors.HedgeError) {
	return dbClient.client.AcquireRedisLock(name)
}
