package cache

import (
	"hedge/common/client"
	"hedge/common/dto"
	"context"
	"encoding/json"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/patrickmn/go-cache"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRateLimit       = 100
	defaultRefreshInterval = 4 * 60.0 * 60
)

type MemCache struct {
	service         interfaces.ApplicationService
	client          client.HTTPClient
	cacheMemDevice  *cache.Cache
	cacheMemProfile *cache.Cache
	deviceExtnUrl   string
	coreMetaDataUrl string
	nodeId          string
	refreshInterval float64
	sharedLockMutex sync.Mutex
	locks           map[string]*sync.Mutex
	requestLimiter  *time.Ticker
}

type Manager interface {
	FetchDeviceTags(device string) (interface{}, bool)
	FetchProfileTags(profile string) (interface{}, bool)
	FetchDownSamplingConfiguration(profile string) (interface{}, bool)
	FetchRawDataConfiguration() (interface{}, bool)

	UpdateDeviceTags(deviceName string, forceFetch bool)
	UpdateProfileTags(profileName string, forceFetch bool)
	UpdateProfileDownSampling(profileName string, forceFetch bool)
	UpdateNodeRawData(forceFetch bool)
}

func NewMemCache(service interfaces.ApplicationService, client client.HTTPClient, nodeId string) (Manager, error) {
	mc := new(MemCache)
	mc.service = service
	mc.nodeId = nodeId
	mc.client = client
	mc.locks = make(map[string]*sync.Mutex)

	// set requests rate limit per second
	mc.requestLimiter = time.NewTicker(time.Second / defaultRateLimit)

	mc.refreshInterval = defaultRefreshInterval
	cacheRefreshIntervalSecsStr, err := service.GetAppSetting("CacheRefreshIntervalSecs")
	if err == nil {
		if refreshInterval, err := strconv.ParseFloat(cacheRefreshIntervalSecsStr, 64); err == nil && refreshInterval > 10 {
			service.LoggingClient().Infof("cacheRefreshIntervalSecsStr=%f (secs)", refreshInterval)
			mc.refreshInterval = refreshInterval
		}
	}

	mc.cacheMemDevice = cache.New(time.Duration(mc.refreshInterval)*time.Second, time.Duration(mc.refreshInterval+1)*time.Second)
	mc.cacheMemProfile = cache.New(time.Duration(mc.refreshInterval)*time.Second, time.Duration(mc.refreshInterval+1)*time.Second)

	deviceExtnUrl, err := service.GetAppSetting("Device_Extn")
	if err != nil && len(deviceExtnUrl) != 1 {
		service.LoggingClient().Errorf("failed to retrieve ApiServer from configuration: %s", err)
		return nil, err
	}
	mc.deviceExtnUrl = deviceExtnUrl

	metadataUrl, err := service.GetAppSetting("MetaDataServiceUrl")
	if err != nil && len(metadataUrl) != 1 {
		service.LoggingClient().Errorf("failed to retrieve ApiServer from configuration: %s", err)
		return nil, err
	}
	mc.coreMetaDataUrl = metadataUrl

	return mc, nil
}

func (mc *MemCache) UpdateDeviceTags(deviceName string, forceFetch bool) {
	// get the lock for this specific device
	mc.sharedLockMutex.Lock()
	deviceLock, exists := mc.locks[deviceName]
	if !exists {
		deviceLock = &sync.Mutex{}
		mc.locks[deviceName] = deviceLock
	}
	mc.sharedLockMutex.Unlock()

	// lock only the current device
	deviceLock.Lock()
	defer deviceLock.Unlock()

	// rate limit request per second
	<-mc.requestLimiter.C

	if _, found := mc.cacheMemDevice.Get(deviceName); found && !forceFetch {
		mc.service.LoggingClient().Debugf("device: %s throttled, will skip the metric data", deviceName)
		return
	}

	var deviceTags = make(map[string]interface{})
	// get business data
	bizData := mc.getBizData(deviceName)
	for k, v := range bizData {
		deviceTags[k] = v
	}

	// get device attributes
	deviceAttrData := mc.getDeviceAttributesData(deviceName)
	for k, v := range deviceAttrData {
		deviceTags[k] = v
	}

	// get device labels
	rsp, err := mc.service.DeviceClient().DeviceByName(context.Background(), deviceName)
	if err == nil && len(rsp.Device.Labels) > 0 {
		deviceLabels := strings.Join(rsp.Device.Labels, ",")
		deviceTags["devicelabels"] = deviceLabels
	}

	expiration := mc.newExpiration()
	mc.cacheMemDevice.Set(deviceName, deviceTags, expiration)
	mc.service.LoggingClient().Infof("Device '%s', new cache will expire in %f secs (tags: %v)", deviceName, expiration.Seconds(), deviceTags)
}

func (mc *MemCache) UpdateProfileTags(profileName string, forceFetch bool) {
	// get the lock for this specific profile
	mc.sharedLockMutex.Lock()
	profileLock, exists := mc.locks[profileName]
	if !exists {
		profileLock = &sync.Mutex{}
		mc.locks[profileName] = profileLock
	}
	mc.sharedLockMutex.Unlock()

	// lock only the current profile
	profileLock.Lock()
	defer profileLock.Unlock()

	// rate limit request per second
	<-mc.requestLimiter.C

	if _, found := mc.cacheMemProfile.Get(profileName); found && !forceFetch {
		mc.service.LoggingClient().Debugf("profile: %s throttled, will skip the metric data", profileName)
		return
	}

	var profileTags = make(map[string]interface{})
	profileData := mc.getProfiledData(profileName)
	for k, v := range profileData {
		profileTags[k] = v
	}

	expiration := mc.newExpiration()
	mc.cacheMemProfile.Set(profileName, profileTags, expiration)
	mc.service.LoggingClient().Infof("Profile '%s', new cache will expire in %f secs (tags: %v)", profileName, expiration.Seconds(), profileTags)
}

func (mc *MemCache) UpdateProfileDownSampling(profileName string, forceFetch bool) {
	// get the lock for this specific profile
	mc.sharedLockMutex.Lock()
	profileLock, exists := mc.locks[profileName]
	if !exists {
		profileLock = &sync.Mutex{}
		mc.locks[profileName] = profileLock
	}
	mc.sharedLockMutex.Unlock()

	// lock only the current profile
	profileLock.Lock()
	defer profileLock.Unlock()

	// rate limit request per second
	<-mc.requestLimiter.C

	if _, found := mc.cacheMemProfile.Get(downSamplingProfileKey(profileName)); found && !forceFetch {
		mc.service.LoggingClient().Debugf("profile sampling: %s throttled, will skip the metric data", profileName)
		return
	}

	expiration := mc.newExpiration()
	mc.cacheMemProfile.Set(downSamplingProfileKey(profileName), mc.getDownSamplingConfig(profileName), expiration)
	mc.service.LoggingClient().Infof("DownSampling for profile '%s' new cache will expire in %f secs", profileName, expiration.Seconds())
}

func (mc *MemCache) UpdateNodeRawData(forceFetch bool) {
	// get the lock for this specific node
	mc.sharedLockMutex.Lock()
	nodeLock, exists := mc.locks[mc.nodeId]
	if !exists {
		nodeLock = &sync.Mutex{}
		mc.locks[mc.nodeId] = nodeLock
	}
	mc.sharedLockMutex.Unlock()

	nodeLock.Lock()
	defer nodeLock.Unlock()

	// rate limit request per second
	<-mc.requestLimiter.C

	if _, found := mc.cacheMemProfile.Get(mc.nodeId); found && !forceFetch {
		mc.service.LoggingClient().Debugf("node raw data: %s throttled, will skip the metric data", mc.nodeId)
		return
	}

	expiration := mc.newExpiration()
	mc.cacheMemProfile.Set(mc.nodeId, mc.getNodeRawDataConfig(), expiration)
	mc.service.LoggingClient().Infof("RawData config for node '%s' new cache will expire in %f secs", mc.nodeId, expiration.Seconds())
}

func (mc *MemCache) FetchDeviceTags(device string) (interface{}, bool) {
	conf, found := mc.cacheMemDevice.Get(device)
	if !found {
		// don't force fetch of tags from the remote if exist in cache already
		mc.UpdateDeviceTags(device, false)
		conf, found = mc.cacheMemDevice.Get(device)
	}
	return conf, found
}

func (mc *MemCache) FetchProfileTags(profile string) (interface{}, bool) {
	conf, found := mc.cacheMemProfile.Get(profile)
	if !found {
		// don't force fetch of tags from the remote if exist in cache already
		mc.UpdateProfileTags(profile, false)
		conf, found = mc.cacheMemProfile.Get(profile)
	}
	return conf, found
}

func (mc *MemCache) FetchDownSamplingConfiguration(profile string) (interface{}, bool) {
	key := downSamplingProfileKey(profile)
	conf, found := mc.cacheMemProfile.Get(key)
	if !found {
		// don't force fetch of tags from the remote if exist in cache already
		mc.UpdateProfileDownSampling(profile, false)
		conf, found = mc.cacheMemProfile.Get(key)
	}
	return conf, found
}

func (mc *MemCache) FetchRawDataConfiguration() (interface{}, bool) {
	conf, found := mc.cacheMemProfile.Get(mc.nodeId)
	if !found {
		// don't force fetch of tags from the remote if exist in cache already
		mc.UpdateNodeRawData(false)
		conf, found = mc.cacheMemProfile.Get(mc.nodeId)
	}
	return conf, found
}

func (mc *MemCache) getBizData(deviceName string) map[string]interface{} {
	tags := make(map[string]interface{})
	req, _ := http.NewRequest("GET", mc.deviceExtnUrl+"/api/v3/metadata/device/biz/"+deviceName, nil)
	resp, err := mc.client.Do(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		mc.service.LoggingClient().Debugf("response from Business Data is null, device: %s", deviceName)
		return tags
	}
	defer resp.Body.Close()

	bizDataByte, err := io.ReadAll(resp.Body)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed reading Business Data from API device: %s error: %s", deviceName, err)
		return tags
	}

	var bizData map[string]interface{}
	err = json.Unmarshal(bizDataByte, &bizData)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing Business Data from API device: %s error: %s", deviceName, err)
		return tags
	}

	for k, v := range bizData {
		if v != nil {
			tags[k] = v
		}
	}
	return tags
}

func (mc *MemCache) getProfiledData(profileName string) map[string]interface{} {
	tags := make(map[string]interface{})
	req, _ := http.NewRequest("GET", mc.coreMetaDataUrl+"/api/v3/deviceprofile/name/"+profileName, nil)
	resp, err := mc.client.Do(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		mc.service.LoggingClient().Debugf("failed to get profile info from metadata, profile: %s error: %s", profileName, err)
		return tags
	}
	defer resp.Body.Close()

	deviceProfileByte, err := io.ReadAll(resp.Body)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing Device Profile from API profile: %s error: %s", profileName, err)
		return tags
	}
	if resp.StatusCode != 200 {
		mc.service.LoggingClient().Debugf("error response when fetching Device Profile from API. status: %s. (%s)", resp.Status, string(deviceProfileByte))
		return tags
	}

	var deviceData dto.ProfileObject
	err = json.Unmarshal(deviceProfileByte, &deviceData)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing Device Profile from API profile name: %s error: %s", profileName, err)
		return tags
	}

	tags["model"] = deviceData.Profile.Model
	tags["manufacturer"] = deviceData.Profile.Manufacturer
	return tags
}

func (mc *MemCache) getDeviceAttributesData(deviceName string) map[string]interface{} {
	tags := make(map[string]interface{})
	req, _ := http.NewRequest("GET", mc.deviceExtnUrl+"/api/v3/metadata/device/extn/"+deviceName, nil)
	resp, err := mc.client.Do(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		mc.service.LoggingClient().Errorf("failed to GET Device Attributes from API, error: %s", err)
		return tags
	}
	defer resp.Body.Close()

	attrByte, err := io.ReadAll(resp.Body)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing Device Attributes from API device: %s error: %s", deviceName, err)
		return tags
	}

	var deviceAttrArr []map[string]interface{}
	err = json.Unmarshal(attrByte, &deviceAttrArr)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing Device Attributes from API device: %s error: %s", deviceName, err)
		return tags
	}

	for _, deviceAttr := range deviceAttrArr {
		key := deviceAttr["field"].(string)
		val := deviceAttr["value"]
		if _, ok := deviceAttr["default"]; val == nil && ok {
			val = deviceAttr["default"]
		}
		if val != nil {
			tags[key] = val
		}
	}
	return tags
}

func (mc *MemCache) getDownSamplingConfig(profile string) dto.DownsamplingConfig {
	downSamplingConfig := dto.DownsamplingConfig{
		Aggregates: nil,
	}

	req, _ := http.NewRequest("GET", mc.deviceExtnUrl+"/api/v3/metadata/deviceprofile/"+profile+"/downsampling/config", nil)
	resp, err := mc.client.Do(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		mc.service.LoggingClient().Errorf("failed getting down sampling config from device extenstions err: %s", err)
		return downSamplingConfig
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed reading down sampling config data for profile: %s error: %s", profile, err)
		return downSamplingConfig
	}

	var config dto.DownsamplingConfig
	err = json.Unmarshal(bodyBytes, &config)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing down sampling config data for profile: %s error: %s", profile, err)
		return downSamplingConfig
	}
	return config
}

func (mc *MemCache) getNodeRawDataConfig() dto.NodeRawDataConfig {
	nodeRawDataConfig := dto.NodeRawDataConfig{
		SendRawData: false,
	}

	req, _ := http.NewRequest("GET", mc.deviceExtnUrl+"/api/v3/node_mgmt/downsampling/rawdata", nil)
	query := req.URL.Query()
	query.Add("nodes", mc.nodeId)
	req.URL.RawQuery = query.Encode()

	resp, err := mc.client.Do(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK {
		mc.service.LoggingClient().Errorf("failed getting raw data config for node: %s", mc.nodeId)
		return nodeRawDataConfig
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed reading raw data config for node: %s error: %s", mc.nodeId, err)
		return nodeRawDataConfig
	}

	var confByNodeId map[string]dto.NodeRawDataConfig
	err = json.Unmarshal(bodyBytes, &confByNodeId)
	if err != nil {
		mc.service.LoggingClient().Errorf("failed parsing raw data config for node: %s error: %s", mc.nodeId, err)
		return nodeRawDataConfig
	}

	conf, ok := confByNodeId[mc.nodeId]
	if !ok || (ok && conf.SendRawData && conf.EndTime < time.Now().Unix()) {
		conf.SendRawData = false
	}
	// Update the timestamps to Nanoseconds
	conf.EndTime *= 1000000000
	conf.StartTime *= 1000000000

	return conf
}

func downSamplingProfileKey(profileName string) string {
	return "dn:" + profileName
}

func (mc *MemCache) newExpiration() time.Duration {
	// random expiry time set for different devices
	// so all cache refreshes do not get scheduled at the same time - for optimum resource usage
	randomFloat := rand.Float64() //random float between 0-1
	return time.Duration(randomFloat*mc.refreshInterval/2+mc.refreshInterval/2) * time.Second
}
