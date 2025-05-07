/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package util

import (
	dto2 "hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"encoding/json"
	"fmt"
	LOG "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"io"
	"net/http"
	"regexp"
	"unicode/utf8"
)

func GetProfileMetrics(metaDataUrl string, profile string, logger LOG.LoggingClient) ([]string, hedgeErrors.HedgeError) {

	var metrics []string

	errorMessage := fmt.Sprintf("Error getting profile metrics for profile %s", profile)

	profileDetailsWithWrapper, err := GetProfileByName(metaDataUrl, profile, logger)
	if err != nil {
		logger.Errorf("%s: Error getting data: %v", errorMessage, err)
		return metrics, err
	}

	if profileDetailsWithWrapper["profile"] == nil {
		logger.Errorf("%s: No profiles found", errorMessage)
		return metrics, nil
	}
	profileDetails := profileDetailsWithWrapper["profile"].(map[string]interface{})
	if profileDetails["deviceResources"] != nil {
		resources := profileDetails["deviceResources"].([]interface{})
		metrics = make([]string, len(resources))
		for i, resource := range resources {
			resourceMap := resource.(map[string]interface{})
			metrics[i] = resourceMap["name"].(string)
		}
	}

	return metrics, nil
}

func GetProfileByName(metaDataUrl string, profile string, logger LOG.LoggingClient) (map[string]interface{}, hedgeErrors.HedgeError) {
	profileDetailsWithWrapper := make(map[string]interface{})

	errorMessage := fmt.Sprintf("Error retrieving profile %s", profile)

	profileUrl := metaDataUrl + "/api/v3/deviceprofile/name/" + profile
	request, err := http.NewRequest("GET", profileUrl, nil)
	if err != nil {
		logger.Errorf("Error creating request to %s: %v", profileUrl, err)
		return profileDetailsWithWrapper, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := HttpClient.Do(request)
	if err != nil {
		logger.Errorf("Error getting data from %s: %v", profileUrl, err)
		return profileDetailsWithWrapper, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&profileDetailsWithWrapper); err != nil {
		logger.Errorf("Error parsing the response body: %v", err)
		return profileDetailsWithWrapper, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return profileDetailsWithWrapper, nil
}

func DeleteProfileByName(metaDataUrl string, profile string, logger LOG.LoggingClient) hedgeErrors.HedgeError {
	profileUrl := metaDataUrl + "/api/v3/deviceprofile/name/" + profile
	request, err := http.NewRequest("DELETE", profileUrl, nil)

	errorMessage := fmt.Sprintf("Error deleting profile %s", profile)

	if err != nil {
		logger.Errorf("Error creating request: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := HttpClient.Do(request)
	if err != nil {
		logger.Errorf("Error deleting profile %s: %v", profile, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	defer resp.Body.Close()

	var response common.BaseResponse
	body, err := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		logger.Errorf("Failed parsing metadata api response. Error: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	ok := IsBaseResponseSuccess(response, logger)
	if !ok {
		logger.Errorf("Failed deleting the profile %s: %v", profile, response.Message)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s: %s", errorMessage, response.Message))
	}

	return nil
}

func DeleteDeviceByName(metaDataUrl string, device string, logger LOG.LoggingClient) hedgeErrors.HedgeError {
	deviceUrl := metaDataUrl + "/api/v3/device/name/" + device

	errorMessage := fmt.Sprintf("Error deleting device %s", device)

	request, err := http.NewRequest("DELETE", deviceUrl, nil)
	if err != nil {
		logger.Errorf("Error creating request: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := HttpClient.Do(request)
	if err != nil {
		logger.Errorf("Error deleting device %s: %v", device, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer resp.Body.Close()

	var response common.BaseResponse
	body, err := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		logger.Errorf("Failed parsing metadata api response. Error: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	ok := IsBaseResponseSuccess(response, logger)
	if !ok {
		logger.Errorf("Failed deleting the device %s: %s", device, response.Message)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Failed deleting device %s: %s", device, response.Message))
	}

	return nil
}

func GetProfileNames(metaDataUrl string, logger LOG.LoggingClient) ([]string, hedgeErrors.HedgeError) {
	profileList, err := GetProfiles(metaDataUrl, logger)
	if err != nil {
		logger.Errorf("Error getting profile names: %v", err)
		return nil, err
	}
	var profiles []string
	for _, profile := range profileList {
		profileDetails := profile.(map[string]interface{})
		name := profileDetails["name"].(string)
		profiles = append(profiles, name)
	}

	return profiles, nil
}

func GetProfiles(metaDataUrl string, logger LOG.LoggingClient) ([]interface{}, hedgeErrors.HedgeError) {
	profileUrl := metaDataUrl + "/api/v3/deviceprofile/all?limit=-1"

	errorMessage := fmt.Sprintf("Error retreiving all profiles")

	request, err := http.NewRequest("GET", profileUrl, nil)
	if err != nil {
		logger.Errorf("Error creating request: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	resp, err := HttpClient.Do(request)
	if err != nil {
		logger.Errorf("Error getting data: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer resp.Body.Close()

	profileDetails := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&profileDetails); err != nil {
		logger.Errorf("Error parsing the response body: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	profileList := profileDetails["profiles"].([]interface{})
	return profileList, nil
}

func GetProfileFromDevice(metaDataUrl string, deviceName string, logger LOG.LoggingClient) (string, hedgeErrors.HedgeError) {
	profName := ""

	errorMessage := "Failed to get profile from device"

	exists, devc := CheckDeviceExists(metaDataUrl, deviceName, logger)
	if !exists {
		logger.Errorf("Device %s not found", deviceName)
		return profName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device %s not found", errorMessage, deviceName))
	}

	//error out if mandatory fields (from profile) are not provided, unless profile had defaults for those fields
	if _, ok := devc["device"]; !ok {
		errStr := fmt.Sprintf("%s: Profile information not found for this device %s", errorMessage, deviceName)
		logger.Errorf(errStr)
		return profName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errStr)
	}

	deviceMap := devc["device"].(map[string]interface{})
	if _, ok := deviceMap["profileName"]; !ok {
		errStr := fmt.Sprintf("%s: Profile information not found for this device %s", errorMessage, deviceName)
		logger.Errorf(errStr)
		return profName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errStr)
	}

	profName = deviceMap["profileName"].(string)
	return profName, nil
}

func CheckProfileExists(metaDataUrl string, profileName string, logger LOG.LoggingClient) (bool, map[string]interface{}, hedgeErrors.HedgeError) {
	profile, err := GetProfileByName(metaDataUrl, profileName, logger)
	ok := isMetaResponseSuccess(profile, err, logger)
	return ok, profile, err
}

func CheckDeviceExists(metaDataUrl string, deviceName string, logger LOG.LoggingClient) (bool, map[string]interface{}) {
	device, err := getDeviceByName(metaDataUrl, deviceName, logger)
	ok := isMetaResponseSuccess(device, err, logger)
	return ok, device
}

func isMetaResponseSuccess(metaResponse map[string]interface{}, hedgeError hedgeErrors.HedgeError, logger LOG.LoggingClient) bool {
	if hedgeError != nil {
		logger.Errorf("Meta response success check failed: %v", hedgeError)
		return false
	}

	var response common.BaseResponse
	jsonBytes, err := json.Marshal(metaResponse)
	if err != nil {
		logger.Errorf("Failed marshaling the metadata response into BaseResponse: %v", err)
		return false
	}

	err = json.Unmarshal(jsonBytes, &response)
	if err != nil {
		logger.Errorf("Failed unmarshaling the metadata response to edgex BaseResponse: %v", err)
		//Probably not an Error Response object. Return success
		return true
	}

	if !isValidResponse(response) {
		logger.Errorf("Invalid characters in metaResponse: %v", response)
		return false
	}

	return IsBaseResponseSuccess(response, logger)
}

func isValidResponse(resp common.BaseResponse) bool {
	allowedMessageSymbols := regexp.MustCompile(`^[a-zA-Z0-9 .,;!?\-_\[\]{}()]*$`)
	if !allowedMessageSymbols.MatchString(resp.Message) {
		return false
	}
	if utf8.RuneCountInString(resp.RequestId) > 255 {
		return false
	}
	allowedRequestIdSymbols := regexp.MustCompile(`^[a-zA-Z0-9\-_]*$`)
	if !allowedRequestIdSymbols.MatchString(resp.RequestId) {
		return false
	}
	return true
}

func IsBaseResponseSuccess(metaResponse common.BaseResponse, logger LOG.LoggingClient) bool {
	var HttpSuccess = 200

	if metaResponse.StatusCode != HttpSuccess {
		errStr := fmt.Sprintf("Failure response. Error: [%d] %s", metaResponse.StatusCode, metaResponse.Message)
		logger.Error(errStr)
		return false
	}

	return true
}

func getDeviceByName(metaDataUrl string, device string, logger LOG.LoggingClient) (map[string]interface{}, hedgeErrors.HedgeError) {

	deviceDetailsWithWrapper := make(map[string]interface{})

	errorMessage := fmt.Sprintf("Error getting device %s", device)

	deviceUrl := metaDataUrl + "/api/v3/device/name/" + device
	request, err := http.NewRequest("GET", deviceUrl, nil)
	if err != nil {
		logger.Errorf("Error creating request at %s: %v", deviceUrl, err)
		err := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		return deviceDetailsWithWrapper, err
	}
	request.Header.Set("Content-type", "application/json")
	resp, err := HttpClient.Do(request)
	if err != nil {
		logger.Errorf("Error getting data from %s: %v", deviceUrl, err)
		err := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		return deviceDetailsWithWrapper, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&deviceDetailsWithWrapper); err != nil {
		logger.Errorf("Error parsing the response body from %s: %v", deviceUrl, err)
		err := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		return deviceDetailsWithWrapper, err
	}

	return deviceDetailsWithWrapper, nil
}

func IfMandatoryFieldsExists(profExts []dto2.DeviceExtension, deviceExts []dto2.DeviceExt, logger LOG.LoggingClient) ([]dto2.DeviceExt, hedgeErrors.HedgeError) {
	for _, profExt := range profExts {
		//mandatory ext field in profile (with no defaults) should be necessarily created in device ext
		if profExt.IsMandatory {
			var fieldExists = false
			for _, deviceExt := range deviceExts {
				if deviceExt.Field == profExt.Field {
					fieldExists = true
					break
				}
			}

			if !fieldExists && len(profExt.Default) == 0 {
				errStr := fmt.Sprintf("Mandatory attribute \"%s\" missing in Device Attributes", profExt.Field)
				logger.Errorf(errStr)
				return deviceExts, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, errStr)
			}
		}
	}
	return deviceExts, nil
}

func IfDeviceFieldsExistsInProfile(profExts []dto2.DeviceExtension, deviceExts []dto2.DeviceExt, logger LOG.LoggingClient) (bool, hedgeErrors.HedgeError) {
	for _, deviceExt := range deviceExts {
		exists := false
		for _, profExt := range profExts {
			if deviceExt.Field == profExt.Field {
				exists = true
				break
			}
		}

		if !exists {
			errStr := fmt.Sprintf("Device field \"%s\" missing in profile ext", deviceExt.Field)
			logger.Errorf(errStr)
			return false, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errStr)
		}
	}

	return true, nil
}

/*func validateDeviceExts(metadataUrl string, deviceName string, deviceExts []dto.DeviceExt) ([]dto.DeviceExt, error) {

	profileName, err := getProfileFromDevice(metadataUrl, deviceName)
	if err != nil {
		return deviceExts, err
	}

	profExts, err := metaSvc.dbClient.GetProfileExt(profileName)
	if err != nil {
		errStr := fmt.Sprintf("Failed to fetch profile extensions for profile %s, device %s", profileName, deviceName)
		metaSvc.service.LoggingClient().Error(errStr)
		return deviceExts, errors.New(errStr)
	}

	//TODO: remove default and isMandatory from Device ext post/put

	//mandatory profile fields (without default values) should be overridden
	deviceExts, err = util.IfMandatoryFieldsExists(profExts, deviceExts)
	if err != nil {
		return deviceExts, err
	}

	//device cannot pass extension fields not in profile ext
	if ok, err := util.IfDeviceFieldsExistsInProfile(profExts, deviceExts); !ok {
		return deviceExts, err
	}

	return deviceExts, nil
}*/

func DeviceExt2Response(deviceExts []dto2.DeviceExt) []dto2.DeviceExtResp {
	var deviceExtsResp = make([]dto2.DeviceExtResp, 0)
	for _, deviceExt := range deviceExts {
		deviceExtsResp = append(deviceExtsResp, dto2.DeviceExtResp{Field: deviceExt.Field, Value: deviceExt.Value})
	}

	return deviceExtsResp
}

func Resp2DeviceExt(deviceExtResps []dto2.DeviceExtResp) []dto2.DeviceExt {
	var deviceExts = make([]dto2.DeviceExt, 0)
	for _, deviceExtResp := range deviceExtResps {
		deviceExts = append(deviceExts, dto2.DeviceExt{Field: deviceExtResp.Field, Value: deviceExtResp.Value})
	}

	return deviceExts
}
