/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"

	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/startup"

	"hedge/common/client"
	"hedge/common/models"
)

type DeviceInfoService struct {
	// edgexSdk               *appsdk.AppFunctionsSDK
	metaDataServiceUrl     string
	deviceToDeviceInfoMap  map[string]models.DeviceInfo
	metricToDeviceInfoList map[string][]models.DeviceInfo
	labels                 []string
	profiles               []string
	profileToDeviceMap     map[string][]string
	labelToDeviceMap       map[string][]string
}

var deviceInfoService *DeviceInfoService

func newDeviceInfoService(metaDataServiceUrl string) *DeviceInfoService {
	deviceInfoSvc := new(DeviceInfoService)
	deviceInfoSvc.metaDataServiceUrl = metaDataServiceUrl

	deviceToDeviceInfoMap, metricToDeviceInfoList := checkMetadataConnection(deviceInfoSvc)
	deviceInfoSvc.deviceToDeviceInfoMap = deviceToDeviceInfoMap
	deviceInfoSvc.metricToDeviceInfoList = metricToDeviceInfoList
	deviceInfoService = deviceInfoSvc
	return deviceInfoSvc
}

func checkMetadataConnection(
	deviceInfoService *DeviceInfoService,
) (deviceToDeviceInfoMap map[string]models.DeviceInfo, metricToDeviceInfoMap map[string][]models.DeviceInfo) {
	var err error
	startupTimer := startup.NewStartUpTimer(client.ServiceKeyHedgePrefix)
	for startupTimer.HasNotElapsed() {
		deviceToDeviceInfoMap, metricToDeviceInfoMap, err = deviceInfoService.GetDeviceInfoMap()
		if err == nil {
			break
		}
		deviceToDeviceInfoMap = nil
		startupTimer.SleepForInterval()
	}
	if err != nil {
		fmt.Println(
			"Failed to connect to edgex core metadata service in allotted time, or there was error from core-metadata",
		)
		os.Exit(1)
	}
	return deviceToDeviceInfoMap, metricToDeviceInfoMap
}

func GetDeviceInfoService(metaDataServiceUrl string) *DeviceInfoService {
	var url string
	if len(metaDataServiceUrl) == 0 {
		url = "http://edgex-core-metadata:59881"
	} else {
		url = metaDataServiceUrl
	}
	deviceInfoService = newDeviceInfoService(url)
	return deviceInfoService
}

func (ds *DeviceInfoService) LoadProfileAndLabels() *DeviceInfoService {
	profiles, labels, profileMap, labelMap := ds.getProfileAndLabelMaps()
	ds.labelToDeviceMap = labelMap
	ds.profileToDeviceMap = profileMap
	ds.labels = labels
	ds.profiles = profiles
	return ds
}

func (ds *DeviceInfoService) GetDeviceProfiles() []string {
	var profiles []string

	for _, device := range ds.deviceToDeviceInfoMap {
		if !isValueExists(profiles, device.ProfileName) {
			profiles = append(profiles, device.ProfileName)
		}
	}
	return profiles
}

func (ds *DeviceInfoService) GetDeviceLabels() []string {
	var labels []string
	for _, device := range ds.deviceToDeviceInfoMap {
		for _, label := range device.Labels {
			if !isValueExists(labels, label) {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

func isValueExists(listValues []string, newValue string) bool {
	for _, eachKey := range listValues {
		if eachKey == newValue {
			return true
		}
	}
	return false
}

func (ds *DeviceInfoService) GetDevicesByLabels(labels []string) []string {

	var devices []string
	devicesMap := make(map[string]int, 0)
	for _, label := range labels {
		for _, device := range ds.labelToDeviceMap[label] {
			devicesMap[device]++
		}
	}
	for device, noofRepeats := range devicesMap {
		if noofRepeats == len(labels) {
			devices = append(devices, device)
		}
	}
	sort.Strings(devices)
	return devices
}

func (ds *DeviceInfoService) GetDevicesByLabelsCriteriaOR(labels []string) []string {

	var devices []string
	devicesMap := make(map[string]int, 0)
	for _, label := range labels {
		for _, device := range ds.labelToDeviceMap[label] {
			devicesMap[device]++
		}
	}
	for device := range devicesMap {
		devices = append(devices, device)
	}
	sort.Strings(devices)

	return devices
}

func (ds *DeviceInfoService) GetDeviceToDeviceInfoMap() map[string]models.DeviceInfo {
	return ds.deviceToDeviceInfoMap
}

/*func (ds *DeviceInfoService) GetMetricToDeviceInfoList() map[string][]models.DeviceInfo {
	return ds.metricToDeviceInfoList
}*/

func (ds *DeviceInfoService) GetLabels() []string {
	if len(ds.labels) == 0 {
		ds.LoadDeviceInfoFromDB()
	}
	return ds.labels
}

func (ds *DeviceInfoService) GetProfiles() []string {
	if len(ds.profiles) == 0 {
		ds.LoadDeviceInfoFromDB()
	}
	return ds.profiles
}

func (ds *DeviceInfoService) GetDevicesByProfile(profile string) []string {
	devices := ds.profileToDeviceMap[profile]
	sort.Strings(devices)
	return devices
}

func (ds *DeviceInfoService) GetDevicesByLabel(label string) []string {
	devices := ds.labelToDeviceMap[label]
	sort.Strings(devices)
	return devices
}

func (ds *DeviceInfoService) GetMetricsByDevices(devices []string) []string {
	var metrices []string

	for _, deviceName := range devices {
		device := ds.deviceToDeviceInfoMap[deviceName]
		for _, metric := range device.Metrices {
			if !isValueExists(metrices, metric) {
				metrices = append(metrices, metric)
			}
		}
	}
	sort.Strings(metrices)
	return metrices
}

func (ds *DeviceInfoService) GetDeviceInfoMap() (deviceToDeviceInfoMap map[string]models.DeviceInfo, metricToDeviceInfoMap map[string][]models.DeviceInfo, err error) {
	deviceToDeviceInfoMap = make(map[string]models.DeviceInfo)
	metricToDeviceInfoMap = make(map[string][]models.DeviceInfo)

	baseUrl := ds.metaDataServiceUrl + "/api/v3/device/all"
	limit := 100 // Number of devices to fetch per request (adjust as needed)
	offset := 0  // Start from the first device

	// Start the paging loop
	for {
		// Construct the URL with pagination parameters
		url := fmt.Sprintf("%s?limit=%d&offset=%d", baseUrl, limit, offset)

		// Prepare the HTTP request
		var data []byte
		req, err := http.NewRequest("GET", url, bytes.NewBuffer(data))
		if err != nil {
			return nil, metricToDeviceInfoMap, fmt.Errorf("error creating request: %v", err)
		}
		req.Header.Add("Content-Type", "application/json")

		// Make the API request
		resp, err := client.Client.Do(req)
		if err != nil {
			return nil, metricToDeviceInfoMap, fmt.Errorf("error getting the device list: %v", err)
		}
		defer resp.Body.Close()

		// Check for successful response
		if resp.StatusCode == 416 {
			return deviceToDeviceInfoMap, metricToDeviceInfoMap, nil
		}

		if resp.StatusCode != 200 {
			return nil, metricToDeviceInfoMap, fmt.Errorf(
				"error getting device list, status code: %d",
				resp.StatusCode,
			)
		}

		// Parse the response
		r := make(map[string]interface{}, 0)
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, metricToDeviceInfoMap, fmt.Errorf(
				"error parsing the response body: %v",
				err,
			)
		}

		// Extract devices from the response
		devices := r["devices"].([]interface{})
		if len(devices) == 0 {
			// If no devices, exit the loop
			break
		}

		// Process the devices
		for _, device := range devices {
			deviceDetails := device.(map[string]interface{})
			deviceName := deviceDetails["name"].(string)
			deviceInfo := models.DeviceInfo{
				Name:        deviceName,
				Labels:      nil,
				ProfileName: "",
			}

			// Parse device labels
			if deviceDetails["labels"] != nil {
				labelsInterface := deviceDetails["labels"].([]interface{})
				if len(labelsInterface) > 0 {
					labels := make([]string, len(labelsInterface))
					for i, label := range labelsInterface {
						labels[i] = label.(string)
					}
					deviceInfo.Labels = labels
				}
			}

			// Parse profile name
			if deviceDetails["profileName"] != nil {
				deviceInfo.ProfileName = deviceDetails["profileName"].(string)
			}

			// Add to the map of devices
			deviceToDeviceInfoMap[deviceName] = deviceInfo
		}

		if len(devices) < limit {
			break
		}
		offset += limit
	}
	return deviceToDeviceInfoMap, metricToDeviceInfoMap, nil
}

func (ds *DeviceInfoService) getProfileAndLabelMaps() ([]string, []string, map[string][]string, map[string][]string) {
	var profiles []string
	var labels []string
	var profileMap map[string][]string
	var labelMap map[string][]string

	profileMap = make(map[string][]string)
	labelMap = make(map[string][]string)

	for _, device := range ds.deviceToDeviceInfoMap {
		if !isValueExists(profiles, device.ProfileName) {
			profiles = append(profiles, device.ProfileName)
		}
		if !isValueExists(profileMap[device.ProfileName], device.Name) {
			profileMap[device.ProfileName] = append(profileMap[device.ProfileName], device.Name)
		}
		for _, label := range device.Labels {
			if !isValueExists(labels, label) {
				labels = append(labels, label)
			}
			if !isValueExists(labelMap[label], device.Name) {
				labelMap[label] = append(labelMap[label], device.Name)
			}
		}
	}
	sort.Strings(profiles)
	sort.Strings(labels)
	return profiles, labels, profileMap, labelMap
}

func (ds *DeviceInfoService) LoadDeviceInfoFromDB() {
	url := ds.metaDataServiceUrl
	if len(ds.metaDataServiceUrl) == 0 {
		url = "http://edgex-core-metadata:48081"
	}
	deviceInfoService.metaDataServiceUrl = url
	deviceToDeviceInfoMap, metricToDeviceInfoList := checkMetadataConnection(deviceInfoService)
	deviceInfoService.deviceToDeviceInfoMap = deviceToDeviceInfoMap
	deviceInfoService.metricToDeviceInfoList = metricToDeviceInfoList
	deviceInfoService.LoadProfileAndLabels()
}

func (ds *DeviceInfoService) ClearCache() {
	deviceInfoService.labels = []string{}
	deviceInfoService.profiles = []string{}
	deviceInfoService.deviceToDeviceInfoMap = make(map[string]models.DeviceInfo)
	deviceInfoService.metricToDeviceInfoList = make(map[string][]models.DeviceInfo)
	deviceInfoService.profileToDeviceMap = make(map[string][]string)
	deviceInfoService.labelToDeviceMap = make(map[string][]string)
}

// SetDeviceInfoService for testing purpose
func SetDeviceInfoService(svc *DeviceInfoService) {
	deviceInfoService = svc
}

// GetDeviceInfoServiceForTest for testing purpose
func GetDeviceInfoServiceForTest() *DeviceInfoService {
	return deviceInfoService
}
