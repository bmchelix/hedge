/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
)

type DeviceService struct {
	metaDataServiceUrl string
}

var deviceService *DeviceService

func newDeviceService(deviceServiceUrl string) *DeviceService {
	deviceSrv := new(DeviceService)
	deviceSrv.metaDataServiceUrl = deviceServiceUrl
	return deviceSrv
}

func GetDeviceService(deviceServiceUrl string) *DeviceService {
	deviceService = nil
	if deviceService == nil {
		var url string
		if len(deviceServiceUrl) == 0 {
			url = "http://edgex-core-metadata:59881"
		} else {
			url = deviceServiceUrl
		}
		url += "/api/v3/deviceservice/all"
		deviceService = newDeviceService(url)
	}
	return deviceService
}

func GetOneDeviceService(deviceServiceUrl string, srvName string) *DeviceService {
	deviceService = nil
	if deviceService == nil {
		var url string
		if len(deviceServiceUrl) == 0 {
			url = "http://edgex-core-metadata:59881"
		} else {
			url = deviceServiceUrl
		}
		url += "/api/v3/deviceservice/name/" + srvName
		deviceService = newDeviceService(url)
	}
	return deviceService
}

func UniqueString(arr []string) []string {
	occurred := map[string]bool{}
	result := []string{}
	for e := range arr {
		// check if variable is set to true or not
		if occurred[arr[e]] != true {
			occurred[arr[e]] = true
			// Append to result slice.
			result = append(result, arr[e])
		}
	}
	return result
}

func BuildDeviceServiceName(
	deviceServiceName string,
	targetNodeType string,
	targetNodeId string,
) string {
	if targetNodeType == "" {
		return deviceServiceName
	}
	if strings.Contains(deviceServiceName, "__") {
		deviceServiceName = strings.Split(deviceServiceName, "__")[1]
	}
	if strings.Contains(targetNodeType, "CORE") {
		if targetNodeId != "" {
			return targetNodeId + "__" + deviceServiceName
		}
	}
	// For target=NODE, just return the raw device serviceName
	return deviceServiceName
}

func SplitDeviceServiceNames(name string) (string, string) {
	tmp := strings.Split(name, "__")
	if len(tmp) == 2 {
		return tmp[0], tmp[1]
	} else {
		return "current", tmp[0]
	}
}

func (ds DeviceService) GetDeviceService() ([]interface{}, hedgeErrors.HedgeError) {
	errorMessage := "Error getting device service"

	url := ds.metaDataServiceUrl
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, err.Error())
	}
	req.Header.Add("Content-Type", "application/json")

	// Get Request details
	resp, err := client.Client.Do(req)
	if err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		fmt.Printf("Error getting the device service list, %d\n", resp.StatusCode)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errorMessage)
	}
	if resp.StatusCode != 200 {
		fmt.Printf("Error getting the device service list, %d\n", resp.StatusCode)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	r := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		fmt.Printf("Error parsing the response body: %v\n", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	switch {
	case r != nil && r["services"] != nil:
		return r["services"].([]interface{}), nil
	case r != nil && r["service"] != nil:
		s := []interface{}{r["service"]}
		r["service"] = s
		return r["service"].([]interface{}), nil
	default:
		return nil, nil
	}

}
