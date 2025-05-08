/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
	"hedge/app-services/hedge-device-extensions/internal/util"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"hedge/common/config"
	db2 "hedge/common/db"
	dto2 "hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	srv "hedge/common/service"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"io"
	"net/http"
	"reflect"
)

type MetaService struct {
	service             interfaces.ApplicationService
	dbClient            redis.DeviceExtDBClientInterface
	dbConfig            *db2.DatabaseConfig
	MetadataUrl         string
	HedgeAdminUrl       string
	CurrentNodeId       string
	attrPattern         *regexp2.Regexp
	profileAliasPattern *regexp2.Regexp
	deviceAliasPattern  *regexp2.Regexp
	profileNamePattern  *regexp2.Regexp
	metricPattern       *regexp2.Regexp
}

func NewMetaService(service interfaces.ApplicationService, dbConfig *db2.DatabaseConfig, dbClient redis.DeviceExtDBClientInterface, metaDataUrl string, currentNodeId string, hedgeAdminUrl string) *MetaService {
	metaService := MetaService{}
	metaService.service = service
	metaService.dbConfig = dbConfig
	metaService.dbClient = dbClient
	metaService.MetadataUrl = metaDataUrl
	metaService.HedgeAdminUrl = hedgeAdminUrl
	metaService.CurrentNodeId = currentNodeId
	metaService.attrPattern = regexp2.MustCompile("(?<=SELECT\\s*)\\w.*\\w(?=(\\s*FROM))", regexp2.IgnoreCase)
	metaService.profileAliasPattern = regexp2.MustCompile("((?<=meta\\(profileName\\)\\s+as\\s+)([^,\\s]+)(?=\\s*(,|$))|meta\\(profileName\\)(?=\\s*(,|$)))", regexp2.IgnoreCase)
	metaService.deviceAliasPattern = regexp2.MustCompile("((?<=meta\\(deviceName\\)\\s+as\\s+)[^,\\s]+(?=\\s*(,|$))|meta\\(deviceName\\)(?=\\s*(,|$)))", regexp2.IgnoreCase)
	//metaService.profileNamePattern = regexp2.MustCompile("(?<=profile\\s*=\\s*\\\\\")\\w*(?=\\\\)", regexp2.None)
	metaService.metricPattern = regexp2.MustCompile("((?<=(?<!meta)\\()[^,)\\s]+(?=\\)\\s*,)|(?<=(?<!meta)\\()[^)]+(?=\\)\\s*(,|$))|(?<=(?<!meta)\\([^)]+\\)\\s+as\\s+)[^,)\\s]+|(?<=(,|^)\\s*)[^,()\\s]+(?=\\s*(,|$))|(?<=(?<!>\\s*[^,()]+|meta\\(\\w+\\))\\s+as\\s+)[^,()\\s]+)", regexp2.IgnoreCase)

	return &metaService
}

// GetSecret is probably not working within time, so adding this to be called later..
func (metaSvc *MetaService) AddDBClient(bool, hedgeErrors.HedgeError) {

}

func (metaSvc *MetaService) AddAssociation(nodeA string, nodeAType string, associationNodes []dto2.AssociationNode) (string, hedgeErrors.HedgeError) {
	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, nodeA, metaSvc.service.LoggingClient())

	errorMessage := fmt.Sprintf("Error adding association for node %s", nodeA)

	if !exists {
		return nodeA, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Node %s not found", nodeA))
	}

	var assId string
	var assNodesMap = make(map[string]string)
	for _, associationNode := range associationNodes {
		//check for duplicates
		if assType, exists := assNodesMap[associationNode.NodeName]; exists && assType == associationNode.NodeType {
			metaSvc.service.LoggingClient().Errorf("%s: Duplicate node passed: %v", errorMessage, associationNode)
			return assId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: Duplicate node passed: %s", errorMessage, associationNode.NodeName))
		}

		assNodesMap[associationNode.NodeName] = associationNode.NodeType
		exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, associationNode.NodeName, metaSvc.service.LoggingClient())
		if !exists {
			return associationNode.NodeName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Node %s not found", errorMessage, associationNode.NodeName))
		}
	}

	//check if primary node is repeated
	if assType, exists := assNodesMap[nodeA]; exists && assType == nodeAType {
		metaSvc.service.LoggingClient().Errorf("Self loop. Primary node passed in association: %v", nodeA)
		return assId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Self loop. Primary node passed in association: "+nodeA)
	}

	metaSvc.service.LoggingClient().Infof("Nodes to add: %v", assNodesMap)

	// Save the Nodes and children in redis database
	return metaSvc.dbClient.AddAssociation(nodeA, nodeAType, associationNodes)
}

func (metaSvc *MetaService) GetAssociation(nodeA string) ([]dto2.AssociationNode, hedgeErrors.HedgeError) {
	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, nodeA, metaSvc.service.LoggingClient())

	errorMessage := fmt.Sprintf("Error getting association for node %s", nodeA)

	if !exists {
		metaSvc.service.LoggingClient().Errorf(fmt.Sprintf("%s: Node %s not found", errorMessage, nodeA))
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Node %s not found", errorMessage, nodeA))
	}

	associationNodes, err := metaSvc.dbClient.GetAssociationByNodeName(nodeA)

	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) && associationNodes == nil {
		associationNodes = make([]dto2.AssociationNode, 0)
		err = nil
	}

	//validate if associations are valid
	var retAssociationNodes = make([]dto2.AssociationNode, 0)

	isDeleted := false
	for _, associationNode := range associationNodes {
		exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, associationNode.NodeName, metaSvc.service.LoggingClient())

		if !exists {
			isDeleted = true
			metaSvc.service.LoggingClient().Errorf("Association node not found (for %s): %s. Possibly deleted. To remove from association", nodeA, associationNode.NodeName)
			continue
		}
		retAssociationNodes = append(retAssociationNodes, associationNode)
	}

	if isDeleted {
		assId, err := metaSvc.dbClient.UpdateAssociation(nodeA, "device", retAssociationNodes)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Error updating the associations for %s - %v. Errpr: %v", nodeA, retAssociationNodes, err)
			return nil, err
		} else {
			metaSvc.service.LoggingClient().Debugf("updated association for %s: %s", nodeA, assId)
		}
	}

	return retAssociationNodes, nil
}

func (metaSvc *MetaService) DeleteAssociation(nodeA string, nodeAType string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting association for node %s", nodeA)

	associations, err := metaSvc.GetAssociation(nodeA)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}
	if len(associations) == 0 {
		metaSvc.service.LoggingClient().Warnf("Association does not exist for %s. Nothing to delete", nodeA)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Not Found", errorMessage))
	}

	metaSvc.service.LoggingClient().Infof("Association to be deleted for %s: %s", nodeA, associations)
	return metaSvc.dbClient.DeleteAssociationByNodeName(nodeA)
}

func (metaSvc *MetaService) UpdateAssociation(nodeA string, nodeAType string, associationNodes []dto2.AssociationNode, forceCreate bool) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error updating association for node %s", nodeA)

	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, nodeA, metaSvc.service.LoggingClient())

	if !exists {
		metaSvc.service.LoggingClient().Errorf("%s: Node %s not found", errorMessage, nodeA)
		return nodeA, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Node %s not found", errorMessage, nodeA))
	}

	var assId string
	var assNodesMap = make(map[string]string)
	for _, associationNode := range associationNodes {
		//check for duplicates
		if assType, exists := assNodesMap[associationNode.NodeName]; exists && assType == associationNode.NodeType {
			metaSvc.service.LoggingClient().Errorf("Duplicate node passed: %v", associationNode)
			return assId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: Duplicate node passed: %s", errorMessage, associationNode.NodeName))
		}

		assNodesMap[associationNode.NodeName] = associationNode.NodeType
		exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, associationNode.NodeName, metaSvc.service.LoggingClient())
		if !exists {
			metaSvc.service.LoggingClient().Errorf("%s: Node %s not found", errorMessage, associationNode.NodeName)
			return associationNode.NodeName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Node %s not found", errorMessage, associationNode.NodeName))
		}
	}

	//check if primary node is repeated
	if assType, exists := assNodesMap[nodeA]; exists && assType == nodeAType {
		metaSvc.service.LoggingClient().Errorf("Self loop. Primary node passed in association: %v", nodeA)
		return assId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: Self loop. Primary node passed in association: %s", errorMessage, nodeA))
	}

	metaSvc.service.LoggingClient().Infof("Nodes to add: %v", assNodesMap)

	// Save the Nodes and children in redis database
	assId, err := metaSvc.dbClient.UpdateAssociation(nodeA, nodeAType, associationNodes)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) && forceCreate {
			metaSvc.service.LoggingClient().Debugf("Device association does not exist. Cannot update. Force creating..")

			//Create instead of Update
			assId, err2 := metaSvc.dbClient.AddAssociation(nodeA, nodeAType, associationNodes)
			if err2 != nil {
				metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
				return assId, err2
			}
		} else if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			errStr := fmt.Sprintf("Device association does not exist for %s", nodeA)
			metaSvc.service.LoggingClient().Errorf("%s: %s", errorMessage, errStr)
			return assId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %s", errorMessage, errStr))
		} else {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return assId, err
		}
	}

	return assId, nil
}

func (metaSvc *MetaService) AddLocation(location models.Location) (locationMap map[string]string, err hedgeErrors.HedgeError) {
	var displayName string

	locationMap = make(map[string]string)
	if location.DisplayName == "" || len(location.DisplayName) == 0 {
		location.DisplayName = location.StreetAddress + "-" + location.City
	}
	displayName = location.DisplayName
	locationId, err := metaSvc.dbClient.AddLocation(location)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error Adding Location: %v", err.Error())
	}
	locationMap["id"] = locationId
	locationMap["displayName"] = displayName
	return locationMap, err
}

func (metaSvc *MetaService) GetLocation(locationId string) (location models.Location, err hedgeErrors.HedgeError) {
	location, err = metaSvc.dbClient.GetLocationById(locationId)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error Getting Location By Id: %v", err.Error())
	}
	return location, err
}

func (metaSvc *MetaService) GetLocations(country string, state string, city string) (locationsMap []map[string]string, err hedgeErrors.HedgeError) {
	var locations []models.Location
	if len(city) != 0 || city != "" {
		locations, err = metaSvc.dbClient.GetLocations(db2.Location + ":city:" + city)
	} else if len(state) != 0 || state != "" {
		locations, err = metaSvc.dbClient.GetLocations(db2.Location + ":state:" + state)
	} else if len(country) != 0 || country != "" {
		locations, err = metaSvc.dbClient.GetLocations(db2.Location + ":country:" + country)
	}

	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error Getting Locations: %v", err.Error())
	}

	locationsMap = make([]map[string]string, len(locations))
	for index, loc := range locations {
		locationsMap[index] = make(map[string]string)
		locationsMap[index]["id"] = loc.Id
		locationsMap[index]["displayName"] = loc.DisplayName
	}

	return locationsMap, err
}

func (metaSvc *MetaService) GetDeviceDetails(deviceName string) (dtos.Device, string, hedgeErrors.HedgeError) {
	var deviceModel dtos.Device
	var version string

	errorMessage := fmt.Sprintf("Error getting device details for device %s", deviceName)

	exists, deviceDetailsWithWrapper := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())

	if !exists {
		metaSvc.service.LoggingClient().Errorf("%s: Device %s not found", errorMessage, deviceName)
		return deviceModel, version, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device %s not found", errorMessage, deviceName))
	}

	return metaSvc.ParseDeviceMap(deviceName, deviceDetailsWithWrapper)
}

func (metaSvc *MetaService) ParseDeviceMap(deviceName string, deviceDetailsWithWrapper map[string]interface{}) (dtos.Device, string, hedgeErrors.HedgeError) {
	var deviceModel dtos.Device

	version := deviceDetailsWithWrapper["apiVersion"].(string)
	if deviceDetailsWithWrapper["device"] == nil {
		metaSvc.service.LoggingClient().Errorf("device field missing in response: %v", deviceDetailsWithWrapper)
		return deviceModel, version, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Internal error during Device fetch "+deviceName)
	}
	deviceDetails := deviceDetailsWithWrapper["device"]
	devcJsonByte, _ := json.Marshal(deviceDetails)
	strings.NewReader(string(devcJsonByte))

	if err := json.Unmarshal(devcJsonByte, &deviceModel); err != nil {
		metaSvc.service.LoggingClient().Errorf("Error fetching device %s: %v", deviceName, err)
		return deviceModel, version, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Internal error during Device fetch "+deviceName)
	}

	return deviceModel, version, nil
}

func (metaSvc *MetaService) GetDevices(query *models.Query) ([]models.DeviceSummary, models.Page, hedgeErrors.HedgeError) {
	var deviceInfo []dtos.Device
	switch query.Filter.Present {
	case true:
		// get device IDs following the filter provided
		deviceIds := metaSvc.GetDevicesIDs(query.Filter)

		// get devices by IDs
		devices, err := metaSvc.dbClient.GetDevicesUsingIds(deviceIds)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Error fetching devices using ids in order %v", err)
			return nil, query.Page, err
		}

		deviceInfo = devices
	default:
		// edgex services are configured to return max 1024 number of results, if you have more devices than that
		// you will need to use the offset and limit to paginate (https://github.com/edgexfoundry/edgex-go/commits/8524b20a)
		limit := 1024
		offset := 0

		for {
			deviceResponse, err := metaSvc.service.DeviceClient().AllDevices(context.Background(), nil, offset, limit)
			if err != nil {
				metaSvc.service.LoggingClient().Error(err.Error())
				return nil, query.Page, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error fetching device summary")
			}

			deviceInfo = append(deviceInfo, deviceResponse.Devices...)

			if len(deviceResponse.Devices) < limit {
				break
			}
			offset += limit
		}
	}

	if query.Filter.SortType != "" && query.Filter.SortBy != "" {
		switch query.Filter.SortType {
		case "asc":
			sort.Slice(deviceInfo, func(i, j int) bool {
				return getField(&deviceInfo[i], query.Filter.SortBy) < getField(&deviceInfo[j], query.Filter.SortBy)
			})
		case "desc":
			sort.Slice(deviceInfo, func(i, j int) bool {
				return getField(&deviceInfo[i], query.Filter.SortBy) > getField(&deviceInfo[j], query.Filter.SortBy)
			})
		}
	}

	// if edgeNode is passed, remove devices from other nodes
	if query.Filter.EdgeNode != "" {
		if query.Filter.EdgeNode == metaSvc.CurrentNodeId {
			deviceInfo = filterByLocalNode(deviceInfo)
		} else {
			deviceInfo = filterByRemoteNode(deviceInfo, query.Filter.EdgeNode)
		}
	}

	query.Page.Total = len(deviceInfo)

	start := (query.Page.Number - 1) * query.Page.Size
	end := query.Page.Number * query.Page.Size

	switch {
	case query.Page.Total < start:
		return nil, query.Page, nil
	case query.Page.Total < end:
		end = query.Page.Total
	}

	return devicesSummary(metaSvc.service, deviceInfo[start:end]), query.Page, nil
}

func (metaSvc *MetaService) GetDevicesIDs(f models.Filter) (filteredResults []string) {
	filterResultMap := make(map[string][]string)
	filteredResults = make([]string, 0)

	//Device
	if f.Device != "" {
		r, err := metaSvc.dbClient.GetDeviceByName(f.Device)
		if err != nil {
			metaSvc.service.LoggingClient().Error("Error fetching Devices " + err.Error())
			return
		}
		filterResultMap["device-results"] = r
	}

	// Profile
	if f.Profile != "" {
		r, err := metaSvc.dbClient.GetFilterIdsByName("md|dv:profile:name:", f.Profile)
		if err != nil {
			metaSvc.service.LoggingClient().Error("Error fetching Profile Ids " + err.Error())
			return
		}
		filterResultMap["profile-results"] = r
	}

	// Service
	if f.Service != "" {
		resp, err := metaSvc.service.DeviceServiceClient().AllDeviceServices(context.Background(), nil, 0, -1)
		if err != nil {
			metaSvc.service.LoggingClient().Error("Error fetching Services " + err.Error())
			return
		}
		for _, s := range resp.Services {
			if strings.HasSuffix(s.Name, f.Service) {
				r, err := metaSvc.dbClient.GetFilterIdsByName("md|dv:service:name:", s.Name)
				if err != nil {
					metaSvc.service.LoggingClient().Error("Error fetching Services " + err.Error())
					return
				}
				filterResultMap["service-results"] = append(filterResultMap["service-results"], r...)
			}
		}
	}

	// Labels Filter
	if f.Labels != "" {
		r, err := metaSvc.dbClient.GetFilterIdsByName("md|dv:label:", f.Labels)
		if err != nil {
			metaSvc.service.LoggingClient().Error("Error fetching Labels " + err.Error())
		}
		filterResultMap["labels-results"] = r
	}

	for _, r := range filterResultMap {
		if len(filteredResults) == 0 && filteredResults != nil {
			filteredResults = r
			continue
		}
		filteredResults = Intersection(filteredResults, r)
	}

	if filteredResults == nil {
		filteredResults = make([]string, 0)
	}

	return filteredResults
}

func filterByRemoteNode(arr []dtos.Device, node string) []dtos.Device {
	var devArr []dtos.Device
	for i := range arr {
		nodeId, _ := srv.SplitDeviceServiceNames(arr[i].ServiceName)
		if strings.Compare(nodeId, node) == 0 {
			devArr = append(devArr, arr[i])
		}
	}
	return devArr
}

func filterByLocalNode(arr []dtos.Device) []dtos.Device {
	var devArr []dtos.Device
	for i := range arr {
		if !strings.Contains(arr[i].ServiceName, "__") {
			devArr = append(devArr, arr[i])
		}
	}
	return devArr
}

func getField(v *dtos.Device, field string) string {
	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field)
	return f.String()
}

func (metaSvc *MetaService) GetMetricsForProfile(profile string) ([]string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error getting metrics for profile %s", profile)

	metrics, err := util.GetProfileMetrics(metaSvc.MetadataUrl, profile, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return metrics, err
	}
	return metrics, nil
}

func (metaSvc *MetaService) GetProtocolsForService(service string) (map[string]string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error getting protocols for service %s", service)

	protocols, err := metaSvc.dbClient.GetProtocolsForService(service)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return protocols, err
	}

	return protocols, nil
}

func (metaSvc *MetaService) CreateCompleteProfile(profileName string, profileObject dto2.ProfileObject) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error creating profile %s", profileName)

	var profileId string
	////////  metadata create profile api
	p := profileObject.Profile
	p.Name = profileName
	newProfile := make(map[string]interface{})
	newProfile["apiVersion"] = profileObject.ApiVersion
	newProfile["profile"] = p

	newProfileArr := append(make([]map[string]interface{}, 0), newProfile)
	jsonByte, err := json.Marshal(newProfileArr)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	payload := strings.NewReader(string(jsonByte))

	req, err := http.NewRequest("POST", metaSvc.MetadataUrl+"/api/v3/deviceprofile", payload)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	client := &http.Client{}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("fail to read the response body: %v", err)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	if resp.StatusCode != 207 {
		metaSvc.service.LoggingClient().Errorf("Error creating a new profile, %d... %s", resp.StatusCode, string(bodyBytes))

		var apiResp common.BaseResponse
		err = json.Unmarshal(bodyBytes, &apiResp)
		if ok := util.IsBaseResponseSuccess(apiResp, metaSvc.service.LoggingClient()); !ok {
			metaSvc.service.LoggingClient().Errorf("Metadata error response during create profile: %v", err)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		} else {
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}
	//207 = multipart success response
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var profRespArr []common.BaseWithIdResponse
	err = decoder.Decode(&profRespArr)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: Failed decoding the profile api response. Error: %v", errorMessage, err)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	var isErr bool = false
	var errMsg string = ""
	for _, profResp := range profRespArr {
		if profResp.StatusCode != 201 {
			metaSvc.service.LoggingClient().Errorf("Failure response during profile creation %s. Error: statusCode=%d, reqId=%s, message=%s", profileName, profResp.StatusCode, profResp.RequestId, profResp.Message)
			errMsg = profResp.Message + ". "
			isErr = true
		} else if len(profResp.Id) > 0 {
			metaSvc.service.LoggingClient().Debugf("Successfully created new profile %s: %s", profileName, profResp.Id)
			profileId = profResp.Id
		}
	}
	// atleast 1 error was returned
	if isErr {
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: %s", errorMessage, errMsg))
	}
	metaSvc.service.LoggingClient().Infof("New profile successfully created %s: %s", profileName, profileId)

	//////// go ahead create profile extension
	if profileObject.DeviceAttributes != nil {
		deviceExtns := profileObject.DeviceAttributes
		if len(deviceExtns) > 0 {
			id, err := metaSvc.AddDeviceExtensionsInProfile(profileName, deviceExtns)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error creating profile extension for %s, error: %v", profileName, err)
				//TODO: revert creation of profile
				return profileId, err
			}
			metaSvc.service.LoggingClient().Infof("Profile extension created for %s: %s", profileName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty profile extension list passed. NOOP")
			/*This may not be needed for Create profile attributes, since attributes will never exist without the profile
			err := metaSvc.DeleteProfileExtension(profileName)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error while deleting profile extension for %s, error: %v", profileName, err)
				return profileId, err
			}
			metaSvc.service.LoggingClient().Infof("Profile extension successfully deleted: %s", profileName)
			*/
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No profile extension found in submitted profile: %s", profileName)
	}

	// Create Downsampling Config: if not provided - set default values
	if profileObject.DownsamplingConfig == nil {
		defaultDownsamplingConfig := &dto2.DownsamplingConfig{
			DefaultDataCollectionIntervalSecs: 20,
			DefaultDownsamplingIntervalSecs:   240,
			Aggregates: []dto2.Aggregate{
				{
					FunctionName:         "avg",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				},
				{
					FunctionName:         "min",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				},
				{
					FunctionName:         "max",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				}},
		}

		err := metaSvc.UpsertDownsamplingConfig(profileName, defaultDownsamplingConfig)
		return profileId, err
	} else {
		err := metaSvc.UpsertDownsamplingConfig(profileName, profileObject.DownsamplingConfig)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Error creating profile defaultDownsamplingConfig for %s, error: %v", profileName, err)
			return profileId, err
		}
	}

	return profileId, nil
}

func (metaSvc *MetaService) UpdateCompleteProfile(profileName string, profileObject dto2.ProfileObject) (string, hedgeErrors.HedgeError) {
	var profileId string

	errorMessage := fmt.Sprintf("Error updating profile %s", profileName)

	exists, profileDetailsWithWrapper, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error fetching profile %s: %v", profileName, err)
		return profileId, err
	} else if !exists {
		metaSvc.service.LoggingClient().Errorf("Profile %s not found", profileName)
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Not Found", errorMessage))
	}

	if profileDetailsWithWrapper["profile"] == nil ||
		profileDetailsWithWrapper["profile"].(map[string]interface{})["id"] == nil {
		metaSvc.service.LoggingClient().Errorf("Failed when parsing the profile object")
		return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	profileId = profileDetailsWithWrapper["profile"].(map[string]interface{})["id"].(string)

	// check if read write field is filled for each metric
	for _, devResource := range profileObject.Profile.DeviceResources {
		if devResource.Properties.ReadWrite == "" {
			metaSvc.service.LoggingClient().Errorf("Error updating profile: Device Resource %s belonging to profile %s requires ReadWrite field", devResource.Name, profileName)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: problem saving this new metric: Read/Write field is required", errorMessage))
		}
	}

	//////// metadata update profile api
	if !profileObject.ProfileIsEmpty() {
		p := profileObject.Profile
		p.Name = profileName
		modProfile := make(map[string]interface{})
		modProfile["apiVersion"] = profileObject.ApiVersion
		modProfile["profile"] = p

		modProfileArr := append(make([]map[string]interface{}, 0), modProfile)
		jsonByte, err := json.Marshal(modProfileArr)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed updating profile %s. Error: %v", profileName, err)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		payload := strings.NewReader(string(jsonByte))

		req, err := http.NewRequest("PUT", metaSvc.MetadataUrl+"/api/v3/deviceprofile", payload)
		if err != nil {
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}

		client := &http.Client{}
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Error updating the profile %s, error: %v", profileName, err)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}

		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("fail to read the response body: %v", err)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		if resp.StatusCode != 207 {
			metaSvc.service.LoggingClient().Errorf("Error updating profile %s, %d... %s", profileName, resp.StatusCode, string(bodyBytes))

			var apiResp common.BaseResponse
			err = json.Unmarshal(bodyBytes, &apiResp)

			if err != nil {
				metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
				return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
			}

			if ok := util.IsBaseResponseSuccess(apiResp, metaSvc.service.LoggingClient()); !ok {
				metaSvc.service.LoggingClient().Errorf("Metadata error response during update profile")
				return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
			}
		}
		//207 = multipart success response
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		var profRespArr []common.BaseWithIdResponse
		err = decoder.Decode(&profRespArr)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed decoding the profile api response. Error: %v", err)
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		var isErr bool = false
		var errMsg string = ""
		for _, profResp := range profRespArr {
			if profResp.StatusCode != 200 {
				metaSvc.service.LoggingClient().Errorf("Failure response during profile updation %s. Error: statusCode=%d, reqId=%s, message=%s", profileName, profResp.StatusCode, profResp.RequestId, profResp.Message)
				errMsg = profResp.Message + ". "
				isErr = true
			} else if len(profResp.Id) > 0 {
				metaSvc.service.LoggingClient().Debugf("Successfully updated %s profile: %s", profileName, profResp.Id)
				profileId = profResp.Id
			}
		}
		// atleast 1 error was returned
		if isErr {
			return profileId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: %s", errorMessage, errMsg))
		}
		metaSvc.service.LoggingClient().Infof("Profile %s successfully updated: %s", profileName, profileId)
	} else {
		metaSvc.service.LoggingClient().Infof("No profile found in submitted profile: %s", profileName)
	}

	//////// go ahead update profile extension
	if profileObject.DeviceAttributes != nil {
		deviceExtns := profileObject.DeviceAttributes
		if len(deviceExtns) > 0 {
			id, err := metaSvc.UpdateDeviceExtensionsInProfile(profileName, deviceExtns, true)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error updating profile extension for %s, error: %v", profileName, err)
				//TODO: revert updation of profile
				return profileId, err
			}
			metaSvc.service.LoggingClient().Infof("Device profile extension updated for %s: %s", profileName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty device profile extension list passed. Removing all..")
			err := metaSvc.DeleteDeviceExtensionInProfile(profileName)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error deleting profile extension for %s, error: %v", profileName, err)
				return profileId, err
			}
			metaSvc.service.LoggingClient().Infof("Device profile extension successfully deleted: %s", profileName)
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No device profile extension found in submitted profile: %s", profileName)
	}

	if profileObject.DownsamplingConfig != nil {
		hedgeError := metaSvc.UpsertDownsamplingConfig(profileName, profileObject.DownsamplingConfig)
		if hedgeError != nil {
			metaSvc.service.LoggingClient().Errorf("Error creating profile DownsamplingConfig for %s, error: %v", profileName, err)
			return profileId, hedgeError
		}
	}
	return profileId, nil
}

func (metaSvc *MetaService) GetCompleteProfile(profileName string) (dto2.ProfileObject, hedgeErrors.HedgeError) {
	var fullProfile dto2.ProfileObject

	errorMessage := fmt.Sprintf("Error getting complete profile %s", profileName)

	//////// Get device Profile
	exists, profileDetailsWithWrapper, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if !exists {
		metaSvc.service.LoggingClient().Errorf("Profile %s not found", profileName)
		return fullProfile, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Not Found", errorMessage))
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error fetching profile %s: %v", profileName, err)
		return fullProfile, err
	}

	if profileDetailsWithWrapper["profile"] == nil {
		metaSvc.service.LoggingClient().Errorf("profile field missing in response: %v", profileDetailsWithWrapper)
		return fullProfile, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	profileDetails := profileDetailsWithWrapper["profile"]
	profJsonByte, _ := json.Marshal(profileDetails)
	strings.NewReader(string(profJsonByte))

	deviceProfile := dtos.DeviceProfile{}
	if err := json.Unmarshal(profJsonByte, &deviceProfile); err != nil {
		metaSvc.service.LoggingClient().Errorf("Error fetching profile %s: %v", profileName, err)
		return fullProfile, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	//////// Get profile extension
	deviceExtns, hedgeError := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
	if hedgeError != nil {
		if hedgeError.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Profile Extension not found. Ignoring..")
		} else {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, hedgeError)
			return fullProfile, hedgeError
		}
	}

	//////// Get profile Contextual attributes
	contextualAttr, hedgeError := metaSvc.dbClient.GetProfileContextualAttributes(profileName)
	if hedgeError != nil {
		if hedgeError.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Profile Contextual attributes not found. Ignoring..")
		} else {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, hedgeError)
			return fullProfile, hedgeError
		}
	}

	//////// Get profile downsampling configuration
	downsamplingConfig, err := metaSvc.dbClient.GetDownsamplingConfig(profileName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Profile downsamplingConfig not found. Creating default one...")
		downsamplingConfig = &dto2.DownsamplingConfig{
			DefaultDataCollectionIntervalSecs: 20,
			DefaultDownsamplingIntervalSecs:   240,
			Aggregates: []dto2.Aggregate{
				{
					FunctionName:         "avg",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				},
				{
					FunctionName:         "min",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				},
				{
					FunctionName:         "max",
					GroupBy:              "device",
					SamplingIntervalSecs: 240,
				}},
		}
		_ = metaSvc.UpsertDownsamplingConfig(profileName, downsamplingConfig)
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return fullProfile, err
	}

	fullProfile.ApiVersion = profileDetailsWithWrapper["apiVersion"].(string)
	fullProfile.Profile = deviceProfile
	fullProfile.DeviceAttributes = deviceExtns
	fullProfile.ContextualAttributes = contextualAttr
	fullProfile.DownsamplingConfig = downsamplingConfig

	return fullProfile, nil
}

func (metaSvc *MetaService) DeleteCompleteProfile(profileName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting complete profile %s", profileName)
	//Check if Profile exists
	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf(err.Error())
		return err
	}

	hasDevices, err := metaSvc.isProfileHasAssociatedDevices(profileName)

	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: checking if profile %s has associated devices failed %v", errorMessage, profileName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %v", errorMessage, err))
	}

	if hasDevices {
		metaSvc.service.LoggingClient().Errorf("%s: profile %s has devices", errorMessage, profileName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: fail to delete the device profile when associated device exists", errorMessage))
	}

	//////// delete profile extension
	//first keep a backup of existing data to revert in case of intermediate failure
	devProfExtExists := true
	devProfExtBackup, err := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Device Profile Extension not found. Continuing ahead..")
			devProfExtExists = false
		} else {
			metaSvc.service.LoggingClient().Errorf(fmt.Sprintf("%s: %v", errorMessage, err))
			return err
		}
	}

	if devProfExtExists {
		err = metaSvc.DeleteDeviceExtensionInProfile(profileName)

		if err != nil {
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Debugf("Profile extension not found. Continuing ahead..")
			} else {
				metaSvc.service.LoggingClient().Errorf("Failed deleting the profile extension for %s: %v", profileName, err.Error())
				return err
			}
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully deleted profile extension for %s", profileName)
		}
	}

	//////// delete profile Contextual attributes
	profContextualAttrExists := true
	profContextualAttrBackup, err := metaSvc.dbClient.GetProfileContextualAttributes(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Profile Contextual attributes not found. Continuing ahead..")
			profContextualAttrExists = false
		} else {
			metaSvc.service.LoggingClient().Errorf("Error getting profile contextual attributes: %v", err)
			return err
		}
	}

	if profContextualAttrExists {
		err = metaSvc.DeleteProfileContextualAttributes(profileName)

		if err != nil {
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Debugf("Profile Contextual attributes not found. Continuing ahead..")
			} else {
				metaSvc.service.LoggingClient().Errorf("Failed deleting the profile Contextual attributes for %s: %v", profileName, err)

				//revert previous deletion steps
				metaSvc.service.LoggingClient().Infof("To recreate device profile extension for %s: %v", profileName, devProfExtBackup)
				profId, err2 := metaSvc.AddDeviceExtensionsInProfile(profileName, devProfExtBackup)
				if err2 != nil {
					metaSvc.service.LoggingClient().Errorf("Failed recreating the profile extension for %s", profileName)
				} else {
					metaSvc.service.LoggingClient().Infof("Successfully recreated the profile extension for %s: %s", profileName, profId)
				}
				//end revert

				return err
			}
		}
	}

	//////// delete profile downsampling config
	//first keep a backup of existing data to revert in case of intermediate failure
	downsamplingConfigExists := true
	downsamplingConfigBackup, err := metaSvc.dbClient.GetDownsamplingConfig(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Profile Downsampling config not found. Continuing ahead..")
			downsamplingConfigExists = false
		} else {
			metaSvc.service.LoggingClient().Errorf(fmt.Sprintf("%s: %v", errorMessage, err))
			return err
		}
	}

	if downsamplingConfigExists {
		err = metaSvc.DeleteDownsamplingConfig(profileName)
		if err != nil {
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Debugf("Profile Downsampling config not found. Continuing ahead..")
			} else {
				metaSvc.service.LoggingClient().Errorf("Failed deleting the Downsampling config for %s: %v", profileName, err)
				return err
			}
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully deleted profile Downsampling config for %s", profileName)
		}
	}

	//////// delete profile
	err = util.DeleteProfileByName(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed deleting the profile %s: %v", profileName, err)

		//revert previous deletion steps
		metaSvc.service.LoggingClient().Infof("To recreate profile Contextual attributes for %s: %v", profileName, profContextualAttrBackup)
		err2 := metaSvc.UpsertProfileContextualAttributes(profileName, profContextualAttrBackup)
		if err2 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the profile Contextual attributes for %s", profileName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the profile Contextual attributes %s", profileName)
		}

		metaSvc.service.LoggingClient().Infof("To recreate device profile extension for %s: %v", profileName, devProfExtBackup)
		profId, err2 := metaSvc.AddDeviceExtensionsInProfile(profileName, devProfExtBackup)
		if err2 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the profile extension for %s", profileName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the profile extension for %s: %s", profileName, profId)
		}

		metaSvc.service.LoggingClient().Infof("To recreate profile downsamplingConfig for %s: %v", profileName, devProfExtBackup)
		err3 := metaSvc.UpsertDownsamplingConfig(profileName, downsamplingConfigBackup)
		if err3 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the profile downsamplingConfig for %s", profileName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the profile downsamplingConfig for %s: %s", profileName, profId)
		}
		//end revert

		return err
	}

	return nil
}

func (metaSvc *MetaService) isProfileHasAssociatedDevices(profile string) (bool, hedgeErrors.HedgeError) {
	query := new(models.Query)

	query.Filter.Profile = profile
	query.Filter.Present = true
	query.Page.Size = 1
	query.Page.Number = 1

	_, page, err := metaSvc.GetDevices(query)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			return false, nil
		}

		metaSvc.service.LoggingClient().Errorf("Error retreiving info about associated devices for profile %s: %v", profile, err)
		return false, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error retreiving info about associated devices for profile %s", profile))
	}

	return page.Total > 0, nil
}

func (metaSvc *MetaService) CreateCompleteDevice(deviceName string, deviceObject dto2.DeviceObject) (string, hedgeErrors.HedgeError) {
	var deviceId string

	errorMessage := fmt.Sprintf("Error creating complete device %s", deviceName)

	//////// metadata create device api
	d := deviceObject.Device
	// If device is for Node, not Core
	if deviceObject.Node.NodeId != metaSvc.CurrentNodeId {
		//d.ServiceName = deviceObject.Device.ServiceName
		d.ServiceName = deviceObject.Node.NodeId + "__" + deviceObject.Device.ServiceName
	}

	d.Name = deviceName
	newDevice := make(map[string]interface{})
	newDevice["apiVersion"] = deviceObject.ApiVersion
	newDevice["device"] = d

	newDeviceArr := append(make([]map[string]interface{}, 0), newDevice)
	jsonByte, err := json.Marshal(newDeviceArr)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed creating new device %s. Error: %v", deviceName, err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	payload := strings.NewReader(string(jsonByte))

	req, err := http.NewRequest("POST", metaSvc.MetadataUrl+"/api/v3/device", payload)
	if err != nil {
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	client := &http.Client{}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error creating a new device %s, error: %v", deviceName, err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("fail to read the response body: %v", err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	if resp.StatusCode != 207 {
		metaSvc.service.LoggingClient().Errorf("Error creating a new device %s, %d... %s", deviceName, resp.StatusCode, string(bodyBytes))

		var apiResp common.BaseResponse
		err = json.Unmarshal(bodyBytes, &apiResp)
		if ok := util.IsBaseResponseSuccess(apiResp, metaSvc.service.LoggingClient()); !ok {
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Metadata error response during create device: %v", err)
				return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}

			metaSvc.service.LoggingClient().Errorf("Metadata error response during create device")
			return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	//207 = multipart success response
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var dvcRespArr []common.BaseWithIdResponse
	err = decoder.Decode(&dvcRespArr)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed decoding the device api response for %s. Error: %v", deviceName, err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	var isErr bool = false
	var errMsg string = ""
	for _, dvcResp := range dvcRespArr {
		if dvcResp.StatusCode != 201 {
			metaSvc.service.LoggingClient().Errorf("Failure response during device creation %s. Error: statusCode=%d, reqId=%s, message=%s", deviceName, dvcResp.StatusCode, dvcResp.RequestId, dvcResp.Message)
			errMsg = dvcResp.Message + ". "
			isErr = true
		} else if len(dvcResp.Id) > 0 {
			metaSvc.service.LoggingClient().Debugf("Successfully created new device %s: %s", deviceName, dvcResp.Id)
			deviceId = dvcResp.Id
		}
	}
	// atleast 1 error was returned
	if isErr {
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: %s", errorMessage, errMsg))
	}
	metaSvc.service.LoggingClient().Infof("New device successfully created %s: %s", deviceName, deviceId)

	//////// go ahead create device extension
	if deviceObject.Extensions != nil {
		deviceExts := util.Resp2DeviceExt(deviceObject.Extensions)
		if len(deviceExts) > 0 {
			id, err := metaSvc.AddDeviceExtension(deviceName, deviceExts)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error creating device extension for %s, error: %v", deviceName, err)
				//TODO: revert creation of device
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device extension created for %s: %s", deviceName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty device extension list passed. NOOP")
			/* This may not be needed for Create device attributes, since attributes will never exist without the device
			err := metaSvc.DeleteAllDeviceExtension(deviceName)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error deleting device extension for %s, error: %v", deviceName, err)
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device extension successfully deleted: %s", deviceName)
			*/
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No device extension found in submitted device %s: %s", deviceName, deviceId)
	}

	//////// go ahead create new association
	if deviceObject.Associations != nil {
		associatedNodes := deviceObject.Associations
		if len(associatedNodes) > 0 {
			id, err := metaSvc.AddAssociation(deviceName, "device", associatedNodes)
			if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				errStr := fmt.Sprintf("Associated device not found: %s", id)
				metaSvc.service.LoggingClient().Errorf(errStr)
				//TODO: revert creating of device + extension
				//return "[Associated Device] " + id, err
				return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %s", errorMessage, errStr))
			} else if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error adding device associations for %s, error: %v", deviceName, err)
				//TODO: revert creating of device + extension
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device association created for %s: %s", deviceName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty associations passed. NOOP.")
			/* This may not be needed for Create association, since association will never exist without the device
			err := metaSvc.DeleteAssociation(deviceName, "device")
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error deleting device associations for %s, error: %v", deviceName, err)
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device association successfully deleted for %s", deviceName)*/
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No device association found in submitted device %s: %s", deviceName, deviceId)
	}

	return deviceId, nil
}

func (metaSvc *MetaService) UpdateCompleteDevice(deviceName string, deviceObject dto2.DeviceObject) (string, hedgeErrors.HedgeError) {
	var deviceId string

	errorMessage := fmt.Sprintf("Failed to update complete device %s", deviceName)

	exists, deviceDetailsWithWrapper := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		errStr := fmt.Sprintf("Device %s not found", deviceName)
		metaSvc.service.LoggingClient().Errorf(errStr)
		//return "[Device] " + deviceId, db2.ErrNotFound
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %s", errorMessage, errStr))
	}

	if deviceDetailsWithWrapper["device"] == nil || deviceDetailsWithWrapper["device"].(map[string]interface{})["id"] == nil {
		metaSvc.service.LoggingClient().Errorf("Failed when parsing the device object")
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	deviceId = deviceDetailsWithWrapper["device"].(map[string]interface{})["id"].(string)
	// Rely on name only, update deviceId with existing deviceId
	deviceObject.Device.Id = deviceId

	//////// metadata update device api
	d := deviceObject.Device
	// If device is for Node, not Core
	if deviceObject.Node.NodeId != metaSvc.CurrentNodeId {
		d.ServiceName = deviceObject.Node.NodeId + "__" + deviceObject.Device.ServiceName
	}
	d.Name = deviceName

	updated := dtos.UpdateDevice{
		Id:             &d.Id,
		Name:           &d.Name,
		Description:    &d.Description,
		AdminState:     &d.AdminState,
		OperatingState: &d.OperatingState,
		ServiceName:    &d.ServiceName,
		ProfileName:    &d.ProfileName,
		Labels:         d.Labels,
		Location:       d.Location,
		AutoEvents:     d.AutoEvents,
		Protocols:      d.Protocols,
		Tags:           d.Tags,
		Properties:     d.Properties,
	}

	var modDevice requests.UpdateDeviceRequest
	modDevice.ApiVersion = deviceObject.ApiVersion
	modDevice.Device = updated

	jsonByte, err := json.Marshal([]requests.UpdateDeviceRequest{modDevice})
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed updating the device %s. Error: %v", deviceName, err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	payload := strings.NewReader(string(jsonByte))

	req, err := http.NewRequest("PATCH", metaSvc.MetadataUrl+"/api/v3/device", payload)
	if err != nil {
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	client := &http.Client{}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error updating the device %s, error: %v", deviceName, err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed to read the response body: %v", err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	if resp.StatusCode != 207 {
		metaSvc.service.LoggingClient().Errorf("Error updating the device %s, %d... %s", deviceName, resp.StatusCode, string(bodyBytes))

		var apiResp common.BaseResponse
		//decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		//err = decoder.Decode(apiResp)
		err = json.Unmarshal(bodyBytes, &apiResp)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed to unmarshal response body: %v", err)
			return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}

		if ok := util.IsBaseResponseSuccess(apiResp, metaSvc.service.LoggingClient()); !ok {
			metaSvc.service.LoggingClient().Errorf("Metadata error response during update device")
			return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
	}
	//207 = multipart success response
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var dvcRespArr []common.BaseWithIdResponse
	err = decoder.Decode(&dvcRespArr)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed decoding the device api response. Error: %v", err)
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	var isErr bool = false
	var errMsg string = ""
	for _, dvcResp := range dvcRespArr {
		if dvcResp.StatusCode != 200 {
			metaSvc.service.LoggingClient().Errorf("Failure response during device updation %s. Error: statusCode=%d, reqId=%s, message=%s", deviceName, dvcResp.StatusCode, dvcResp.RequestId, dvcResp.Message)
			errMsg = dvcResp.Message + ". "
			isErr = true
		} else {
			metaSvc.service.LoggingClient().Debugf("Successfully updated device: %s", deviceName)
		}
	}
	// atleast 1 error was returned
	if isErr {
		return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: %s", errorMessage, errMsg))
	}
	metaSvc.service.LoggingClient().Infof("Device successfully updated: %s", deviceName)

	//////// go ahead update device extension
	if deviceObject.Extensions != nil {
		deviceExts := util.Resp2DeviceExt(deviceObject.Extensions)
		if len(deviceExts) > 0 {
			id, err := metaSvc.UpdateDeviceExtension(deviceName, deviceExts, true)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error updating device extension for %s, error: %v", deviceName, err)
				//TODO: revert updation of device
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device extension updated for %s: %s", deviceName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty device extension list passed. Removing all..")
			err := metaSvc.DeleteAllDeviceExtension(deviceName)
			if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error deleting device extension for %s, error: %v", deviceName, err)
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device extension successfully deleted: %s", deviceName)
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No device extension found in submitted device %s: %s", deviceName, deviceId)
	}

	if deviceObject.ContextualAttributes != nil && len(deviceObject.ContextualAttributes) > 0 {
		metaSvc.service.LoggingClient().Infof("updating contextual attributes for device: %s, value: %v", deviceName, deviceObject.ContextualAttributes)
		metaSvc.UpsertDeviceContextualAttributes(deviceName, deviceObject.ContextualAttributes)
	}

	//////// go ahead update the association
	if deviceObject.Associations != nil {
		associatedNodes := deviceObject.Associations
		if len(associatedNodes) > 0 {
			id, err := metaSvc.UpdateAssociation(deviceName, "device", associatedNodes, true)
			if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				errStr := fmt.Sprintf("Associated device not found: %s", id)
				metaSvc.service.LoggingClient().Errorf(errStr)
				//TODO: revert updating of device + extension
				//return "[Associated Device] " + id, err
				return deviceId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %s", errorMessage, errStr))
			} else if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error updating device associations for %s, error: %v", deviceName, err)
				//TODO: revert updating of device + extension
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device association updated for %s: %s", deviceName, id)
		} else {
			metaSvc.service.LoggingClient().Infof("Empty associations passed. Removing all..")
			err := metaSvc.DeleteAssociation(deviceName, "device")
			if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Infof("Device association not found. Continuing ahead..")
			} else if err != nil {
				metaSvc.service.LoggingClient().Errorf("Error deleting device associations for %s, error: %v", deviceName, err)
				return deviceId, err
			}
			metaSvc.service.LoggingClient().Infof("Device association successfully deleted for %s", deviceName)
		}
	} else {
		metaSvc.service.LoggingClient().Infof("No device association found in submitted device %s: %s", deviceName, deviceId)
	}

	return deviceId, nil
}

func (metaSvc *MetaService) GetCompleteDevice(deviceName string, metrics string, service interfaces.ApplicationService) (dto2.DeviceObject, hedgeErrors.HedgeError) {
	var fullDevice dto2.DeviceObject

	errorMessage := fmt.Sprintf("Error getting complete device %s", deviceName)

	//////// Get Device
	deviceModel, apiVersion, err := metaSvc.GetDeviceDetails(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device not found %s", deviceName)
		return fullDevice, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not found", errorMessage))
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return fullDevice, err
	}

	//////// Get device extension
	deviceExts, err := metaSvc.GetDeviceExtension(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device Extensions not found. Ignoring..")
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return fullDevice, err
	}

	//////// Get device Contextual attributes
	contextualAttr, err := metaSvc.dbClient.GetDeviceContextualAttributes(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device Contextual attributes not found. Ignoring..")
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return fullDevice, err
	}

	//////// Get device associations
	devAssoc, err := metaSvc.GetAssociation(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device association not found. Ignoring..")
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return fullDevice, err
	}

	//////// Get Node
	nodeName, serviceName := srv.SplitDeviceServiceNames(deviceModel.ServiceName)
	deviceModel.ServiceName = serviceName
	node, errC := config.GetNode(service, nodeName)
	if errC != nil {
		metaSvc.service.LoggingClient().Errorf("Error getting node: %v", errC)
		return fullDevice, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: Error getting node %s", errorMessage, nodeName))
	}

	//////// Get metrics
	if metrics != "" {
		profile, err := metaSvc.GetCompleteProfile(deviceModel.ProfileName)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Error getting profile: %v", err)
			return fullDevice, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: Error getting profile %s", errorMessage, deviceModel.ProfileName))
		}
		deviceModel.Properties = make(map[string]interface{})
		deviceModel.Properties["DeviceResources"] = profile.Profile.DeviceResources
	}

	fullDevice.ApiVersion = apiVersion
	fullDevice.Device = deviceModel
	fullDevice.Node = node
	fullDevice.Extensions = deviceExts
	fullDevice.Associations = devAssoc
	fullDevice.ContextualAttributes = contextualAttr

	return fullDevice, nil
}

func (metaSvc *MetaService) DeleteCompleteDevice(deviceName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting complete device %s", deviceName)

	//Check if Device exists
	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		metaSvc.service.LoggingClient().Errorf("Device %s not found", deviceName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device not found", errorMessage))
	}

	//////// delete device extension
	//first keep a backup of existing data to revert incase of intermediate failure
	devcExtExists := true
	devcExtBackup, err := metaSvc.dbClient.GetDeviceExt(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device Extension not found. Continuing ahead..")
		devcExtExists = false
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return err
	}

	if devcExtExists {
		err = metaSvc.DeleteAllDeviceExtension(deviceName)
		if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Device extension not found. Continuing ahead..")
		} else if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed deleting the device extension for %s: %v", deviceName, err.Error())
			return err
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully deleted device extension for %s", deviceName)
		}
	}

	//////// delete device Contextual attributes
	devcContextAttrExists := true
	devcContextAttrBackup, err := metaSvc.dbClient.GetDeviceContextualAttributes(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device Contextual attributes not found. Continuing ahead..")
		devcContextAttrExists = false
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return err
	}

	if devcContextAttrExists {
		err = metaSvc.DeleteDeviceContextualAttributes(deviceName)
		if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Debugf("Device Contextual attributes not found. Continuing ahead..")
		} else if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed deleting the device Contextual attributes for %s: %v", deviceName, err.Error())

			//revert previous deletion steps
			metaSvc.service.LoggingClient().Infof("To recreate device extension for %s: %v", deviceName, devcExtBackup)
			devcId, err2 := metaSvc.AddDeviceExtension(deviceName, devcExtBackup)
			if err2 != nil {
				metaSvc.service.LoggingClient().Errorf("Failed recreating the device extension for %s", deviceName)
			} else {
				metaSvc.service.LoggingClient().Infof("Successfully recreated the device extension for %s: %s", deviceName, devcId)
			}
			//end revert

			return err
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully deleted device Contextual attributes for %s", deviceName)
		}
	}

	//////// delete device association
	devcAssocExists := true
	devcAssocBackup, err := metaSvc.GetAssociation(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device association not found. Continuing ahead..")
		devcAssocExists = false
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return err
	}

	if devcAssocExists {
		err = metaSvc.DeleteAssociation(deviceName, "device")
		if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Infof("Device association not found. Continuing ahead..")
		} else if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed deleting the device association for %s: %s", deviceName, err.Error())

			//revert previous deletion steps
			metaSvc.service.LoggingClient().Infof("To recreate device extension for %s: %v", deviceName, devcExtBackup)
			devcId, err2 := metaSvc.AddDeviceExtension(deviceName, devcExtBackup)
			if err2 != nil {
				metaSvc.service.LoggingClient().Errorf("Failed recreating the device extension for %s", deviceName)
			} else {
				metaSvc.service.LoggingClient().Infof("Successfully recreated the device extension for %s: %s", deviceName, devcId)
			}

			metaSvc.service.LoggingClient().Infof("To recreate device Contextual attributes for %s: %v", deviceName, devcContextAttrBackup)
			err2 = metaSvc.UpsertDeviceContextualAttributes(deviceName, devcContextAttrBackup)
			if err2 != nil {
				metaSvc.service.LoggingClient().Errorf("Failed recreating the device Contextual attributes for %s", deviceName)
			} else {
				metaSvc.service.LoggingClient().Infof("Successfully recreated the device Contextual attributes %s", deviceName)
			}
			//end revert

			return err
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully deleted device association for %s", deviceName)
		}
	}

	//////// delete device
	err = util.DeleteDeviceByName(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Failed deleting the device %s: %v", deviceName, err)

		//revert previous deletion steps
		metaSvc.service.LoggingClient().Infof("To recreate device extension for %s: %v", deviceName, devcExtBackup)
		devcId, err2 := metaSvc.AddDeviceExtension(deviceName, devcExtBackup)
		if err2 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the device extension for %s", deviceName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the device extension for %s: %s", deviceName, devcId)
		}

		metaSvc.service.LoggingClient().Infof("To recreate device Contextual attributes for %s: %v", deviceName, devcContextAttrBackup)
		err2 = metaSvc.UpsertDeviceContextualAttributes(deviceName, devcContextAttrBackup)
		if err2 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the device Contextual attributes for %s", deviceName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the device Contextual attributes %s", deviceName)
		}

		metaSvc.service.LoggingClient().Infof("To recreate device association for %s: %v", deviceName, devcAssocBackup)
		assocId, err2 := metaSvc.AddAssociation(deviceName, "device", devcAssocBackup)
		if err2 != nil {
			metaSvc.service.LoggingClient().Errorf("Failed recreating the device Contextual attributes for %s", deviceName)
		} else {
			metaSvc.service.LoggingClient().Infof("Successfully recreated the device Contextual attributes %s: %s", deviceName, assocId)
		}
		//end revert

		return err
	}

	return nil
}

func (metaSvc *MetaService) AddDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error in adding advice extensions for profile %s", profileName)

	extId := ""
	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		return "", err
	}

	errValidateProfileExts := metaSvc.validateProfileExts(deviceExtns)
	if errValidateProfileExts != nil {
		return extId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("%s: %v", errorMessage, err))
	}

	extId, err = metaSvc.dbClient.AddDeviceExtensionsInProfile(profileName, deviceExtns)
	if err != nil {
		metaSvc.service.LoggingClient().Warn(err.Error())
	}

	return extId, nil
}

func (metaSvc *MetaService) UpdateDeviceExtensionsInProfile(profileName string, deviceExtns []dto2.DeviceExtension, forceCreate bool) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error updating device extensions for profile %s", profileName)

	hedgeError := metaSvc.checkProfileExists(errorMessage, profileName)

	hedgeError = metaSvc.validateProfileExts(deviceExtns)
	if hedgeError != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, hedgeError)
		return "", hedgeErrors.NewCommonHedgeError(hedgeError.ErrorType(), fmt.Sprintf("%s: %s", errorMessage, hedgeError.Message()))
	}

	extId, err := metaSvc.dbClient.UpdateDeviceExtensionsInProfile(profileName, deviceExtns)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) && forceCreate {
		metaSvc.service.LoggingClient().Debugf("Device profile extension does not exist. Cannot update. Force creating..")
		//Create instead of Update
		extId, err = metaSvc.dbClient.AddDeviceExtensionsInProfile(profileName, deviceExtns)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return extId, err
		}

	} else if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		errStr := fmt.Sprintf("Profile extension does not already exist for %s", profileName)
		metaSvc.service.LoggingClient().Error(errStr)
		return extId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, errStr)
	} else if err != nil {
		metaSvc.service.LoggingClient().Error(err.Error())
		return extId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %v", errorMessage, err))
	}

	return extId, nil
}

func (metaSvc *MetaService) GetDeviceExtensionInProfile(profileName string) ([]dto2.DeviceExtension, hedgeErrors.HedgeError) {
	deviceExtns := make([]dto2.DeviceExtension, 0)

	errorMessage := fmt.Sprintf("Error getting device extensions for profile %s", profileName)
	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		return deviceExtns, err
	}

	deviceExtns, err = metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)

	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeDBError) || len(deviceExtns) == 0 {
			metaSvc.service.LoggingClient().Debugf("%s: Device Profile Extensions do not exist for %s", errorMessage, profileName)
			return deviceExtns, err
		} else {
			metaSvc.service.LoggingClient().Error(fmt.Sprintf("%s: %s", errorMessage, err.Error()))
			return deviceExtns, err
		}
	}

	return deviceExtns, nil
}

func (metaSvc *MetaService) DeleteDeviceExtensionInProfile(profileName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting device extensions for profile %s", profileName)

	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		return err
	}

	profExts, err := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) || len(profExts) == 0 {
			metaSvc.service.LoggingClient().Warnf("Profile Extensions does not exist for %s. Nothing to delete", profileName)
			return nil
		} else {
			metaSvc.service.LoggingClient().Warn(err.Error())
			return err
		}
	}

	metaSvc.service.LoggingClient().Infof("Device profile extension to be deleted: %s", profileName)

	return metaSvc.dbClient.DeleteDeviceExtensionInProfile(profileName)
}

func (metaSvc *MetaService) validateProfileExts(deviceExtns []dto2.DeviceExtension) hedgeErrors.HedgeError {
	visited := make(map[string]bool)
	for i, devExt := range deviceExtns {
		if len(devExt.Field) == 0 {
			errStr := fmt.Sprintf("Empty attribute passed #%d", i+1)
			metaSvc.service.LoggingClient().Errorf(errStr)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, errStr)
		}

		if visited[devExt.Field] {
			errStr := fmt.Sprintf("Duplicate attribute passed: %s", devExt.Field)
			metaSvc.service.LoggingClient().Error(errStr)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, errStr)
		} else {
			visited[devExt.Field] = true
		}
	}

	return nil
}

func (metaSvc *MetaService) AddDeviceExtension(deviceName string, deviceExts []dto2.DeviceExt) (string, hedgeErrors.HedgeError) {
	var extId = ""

	errorMessage := fmt.Sprintf("Error adding device extension: %s", deviceName)

	exists, devc := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())

	if !exists {
		return extId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	deviceModel, _, err := metaSvc.ParseDeviceMap(deviceName, devc)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error parsing device Object for %s: %v", deviceName, devc)
		return extId, err
	}

	deviceExts, err = metaSvc.validateDeviceExts(deviceName, deviceExts)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: device extensions validation failed %v", errorMessage, err)
		return extId, err
	}

	extId, err = metaSvc.dbClient.AddDeviceExt(deviceName, deviceModel.ProfileName, deviceExts)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: adding device extensions failed %v", errorMessage, err)
		return extId, err
	}
	return extId, nil
}

func (metaSvc *MetaService) UpdateDeviceExtension(deviceName string, deviceExts []dto2.DeviceExt, forceCreate bool) (string, hedgeErrors.HedgeError) {
	var extId = ""

	errorMessage := fmt.Sprintf("Error updating device extension: %s", deviceName)

	exists, devc := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())

	if !exists {
		return deviceName, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	deviceModel, _, err := metaSvc.ParseDeviceMap(deviceName, devc)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error parsing device Object for %s: %v Error %v", deviceName, devc, err)
		return deviceName, err
	}

	deviceExts, err = metaSvc.validateDeviceExts(deviceName, deviceExts)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: device extensions validation failed %v", errorMessage, err)
		return "", err
	}

	extId, err = metaSvc.dbClient.UpdateDeviceExt(deviceName, deviceModel.ProfileName, deviceExts)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) && forceCreate {
		metaSvc.service.LoggingClient().Debugf("Device extension does not exist. Cannot update. Force creating..")
		//Create instead of Update
		extId, err = metaSvc.dbClient.AddDeviceExt(deviceName, deviceModel.ProfileName, deviceExts)
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return extId, err
		}

	} else if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		errStr := fmt.Sprintf("Device extension does not exist for %s", deviceName)
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, errStr)
		return extId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %v", errorMessage, errStr))
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return extId, err
	}

	return extId, nil
}

func (metaSvc *MetaService) GetDeviceExtension(deviceName string) ([]dto2.DeviceExtResp, hedgeErrors.HedgeError) {
	deviceExts := make([]dto2.DeviceExt, 0)
	deviceExtsResponse := make([]dto2.DeviceExtResp, 0)

	errorMessage := fmt.Sprintf("Error getting device extensions for device %s", deviceName)

	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		return deviceExtsResponse, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	deviceExts, err := metaSvc.dbClient.GetDeviceExt(deviceName)
	deviceExtsResponse = util.DeviceExt2Response(deviceExts)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		metaSvc.service.LoggingClient().Debugf("Device extension does not exist for Device %s. Return empty", deviceName)
		//return deviceExtsResponse, nil
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return deviceExtsResponse, err
	}

	//add missing optional profile ext fields
	profileName, err := util.GetProfileFromDevice(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: Error getting profile from device: %v", errorMessage, err)
		return nil, err
	}

	deviceProfileExts, err := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) || len(deviceProfileExts) == 0 {
		metaSvc.service.LoggingClient().Debugf("%s Profile Extensions does not exist for %s", errorMessage, profileName)
		return deviceExtsResponse, err
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: Error getting device extension in profile %v", errorMessage, err)
		return deviceExtsResponse, err
	}

	newDeviceExtsResp := deviceExtsResponse
	for _, devProfileExt := range deviceProfileExts {
		exists := false
		for i, deviceExt := range deviceExtsResponse {
			if deviceExt.Field == devProfileExt.Field {
				//add mandatory field in response
				if devProfileExt.IsMandatory {
					newDeviceExtsResp[i].IsMandatory = devProfileExt.IsMandatory
				}
				//add default field if it exists
				newDeviceExtsResp[i].Default = devProfileExt.Default
				exists = true
				break
			}
		}

		if !exists {
			newDeviceExtsResp = append(newDeviceExtsResp, dto2.DeviceExtResp{Field: devProfileExt.Field, Default: devProfileExt.Default, IsMandatory: devProfileExt.IsMandatory})
		}
	}

	return newDeviceExtsResp, nil
}

func (metaSvc *MetaService) DeleteAllDeviceExtension(deviceName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting device extension for device %s", deviceName)

	exists, devc := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Extension Not Found", errorMessage))
	}

	dbClients, err := metaSvc.dbClient.GetDeviceExt(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) || len(dbClients) == 0 {
		metaSvc.service.LoggingClient().Warnf("Device Extensions does not exist for %s. Nothing to delete", deviceName)
		return nil
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	metaSvc.service.LoggingClient().Infof("Device extension to be deleted: %s", deviceName)
	deviceModel, _, err := metaSvc.ParseDeviceMap(deviceName, devc)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error parsing device Object for %s: %v", deviceName, devc)
		return err
	}

	return metaSvc.dbClient.DeleteAllDeviceExt(deviceName, deviceModel.ProfileName)
}

func (metaSvc *MetaService) DeleteDeviceExtension(deviceName string, attToDelete []string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting device extension for device %s", deviceName)

	exists, devc := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Attributes not found", errorMessage))
	}

	deviceExtensions, err := metaSvc.dbClient.GetDeviceExt(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) || len(deviceExtensions) == 0 {
		metaSvc.service.LoggingClient().Warnf("Device Attributes does not exist for %s. Nothing to delete", deviceName)
		return nil
	} else if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	metaSvc.service.LoggingClient().Infof("Device attribute to be deleted: %s", deviceName)
	deviceModel, _, err := metaSvc.ParseDeviceMap(deviceName, devc)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("Error parsing device Object for %s: %v", deviceName, devc)
		return err
	}

	var updatedDeviceExt []dto2.DeviceExt
	for _, deviceExt := range deviceExtensions {
		if !slices.Contains(attToDelete, deviceExt.Field) {
			updatedDeviceExt = append(updatedDeviceExt, deviceExt)
		}
	}
	return metaSvc.dbClient.DeleteDeviceExt(deviceName, deviceModel.ProfileName, attToDelete, updatedDeviceExt)
}

func (metaSvc *MetaService) validateDeviceExts(deviceName string, deviceExts []dto2.DeviceExt) ([]dto2.DeviceExt, hedgeErrors.HedgeError) {
	profileName, err := util.GetProfileFromDevice(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if err != nil {
		return deviceExts, err
	}

	devProfExts, err := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) || len(devProfExts) == 0 {
			metaSvc.service.LoggingClient().Debugf("Profile Extensions does not exist for %s, device %s", profileName, deviceName)
			return deviceExts, err
		} else {
			errStr := fmt.Sprintf("Failed to fetch profile attributes for profile %s, device %s", profileName, deviceName)
			metaSvc.service.LoggingClient().Debugf(errStr)
			return deviceExts, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errStr)
		}
	}

	//mandatory profile fields (without default values) should be overridden
	deviceExts, err = util.IfMandatoryFieldsExists(devProfExts, deviceExts, metaSvc.service.LoggingClient())
	if err != nil {
		return deviceExts, err
	}

	//device cannot pass extension fields not in profile ext
	if ok, err := util.IfDeviceFieldsExistsInProfile(devProfExts, deviceExts, metaSvc.service.LoggingClient()); !ok {
		return deviceExts, err
	}

	return deviceExts, nil
}

func (metaSvc *MetaService) checkProfileExists(errorMessage string, profileName string) hedgeErrors.HedgeError {
	exists, _, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())

	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: Error fetching profile %s: %v", errorMessage, profileName, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: Error fetching profile %s", errorMessage, profileName))
	} else if !exists {
		metaSvc.service.LoggingClient().Errorf("%s: Profile %s not found", errorMessage, profileName)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile %s not found", errorMessage, profileName))
	}

	return nil
}

func (metaSvc *MetaService) UpsertProfileContextualAttributes(profileName string, contextualAttrs []string) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("Profile Contextual data to be added for profile: %s", profileName)

	errorMessage := fmt.Sprintf("Error upserting Profile Contextual data for profile: %s", profileName)

	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		return err
	}

	// It is a merge, so merge after getting from DB
	attributes, err := metaSvc.dbClient.GetProfileContextualAttributes(profileName)
	if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		return err
	}
	// Merge the attributes
	if attributes != nil {
		for _, newAttr := range contextualAttrs {
			if !Contains(attributes, newAttr) {
				attributes = append(attributes, newAttr)
			}
		}
	} else {
		attributes = contextualAttrs
	}

	err = metaSvc.dbClient.UpsertProfileContextualAttributes(profileName, attributes)
	if err != nil {
		metaSvc.service.LoggingClient().Warn(err.Error())
	}
	return err
}

func (metaSvc *MetaService) GetProfileContextualAttributes(profileName string) ([]string, hedgeErrors.HedgeError) {
	var data []string

	errorMessage := fmt.Sprintf("Error getting Contextual attributes for profile: %s", profileName)

	metaSvc.service.LoggingClient().Debugf("Getting Profile Contextual data for profile: %s", profileName)

	exists, _, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		return data, err
	} else if !exists {
		metaSvc.service.LoggingClient().Errorf("%s: Profile Not Found", errorMessage)
		return data, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile Not Found", errorMessage))
	}

	data, err = metaSvc.dbClient.GetProfileContextualAttributes(profileName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return data, err
	}

	return data, nil
}

func (metaSvc *MetaService) DeleteProfileContextualAttributes(profileName string) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("Profile Contextual data to be deleted for profile: %s", profileName)

	errorMessage := fmt.Sprintf("Error deleting profile contextual attributes for profile %s", profileName)

	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	if err = metaSvc.dbClient.DeleteProfileContextualAttributes(profileName); err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	return nil
}

func (metaSvc *MetaService) UpsertDeviceContextualAttributes(deviceName string, contextualData map[string]interface{}) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Debugf("Device Contextual data to be added for device: %s", deviceName)

	errorMessage := fmt.Sprintf("Error upserting Device Contextual Attributes for device: %s", deviceName)

	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())

	if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	// If the contextual attribute not present in profile, add it first
	// Find the profile for the device first
	profileName, err := util.GetProfileFromDevice(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}
	contextualAttributes := make([]string, len(contextualData))
	i := 0
	for key := range contextualData {
		contextualAttributes[i] = key
		i++
	}

	metaSvc.UpsertProfileContextualAttributes(profileName, contextualAttributes)

	existingContextualData, err := metaSvc.dbClient.GetDeviceContextualAttributes(deviceName)

	if err == nil {
		// merge the data
		for key, bData := range contextualData {
			existingContextualData[key] = bData
		}
	} else {
		existingContextualData = contextualData
	}

	err = metaSvc.dbClient.UpsertDeviceContextualAttributes(deviceName, existingContextualData)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	return nil
}

func (metaSvc *MetaService) GetDeviceContextualAttributes(deviceName string) (map[string]interface{}, hedgeErrors.HedgeError) {
	var data map[string]interface{}
	metaSvc.service.LoggingClient().Debugf("Getting Device Contextual data for device: %s", deviceName)

	errorMessage := fmt.Sprintf("Error getting Device Contextual Attributes for device: %s", deviceName)

	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		return data, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	data, err := metaSvc.dbClient.GetDeviceContextualAttributes(deviceName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return data, err
	}

	return data, nil
}

func (metaSvc *MetaService) DeleteDeviceContextualAttributes(deviceName string) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("Device Contextual data to be deleted for device: %s", deviceName)

	errorMessage := fmt.Sprintf("Error deleting Device Contextual Attributes for device: %s", deviceName)

	exists, _ := util.CheckDeviceExists(metaSvc.MetadataUrl, deviceName, metaSvc.service.LoggingClient())
	if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Device Not Found", errorMessage))
	}

	if err := metaSvc.dbClient.DeleteDeviceContextualAttributes(deviceName); err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	return nil
}

// GetProfileMetaDataSummary gets profile summary for the query in the form of profile list that is passsed to the function
// Expect this func to be modified to include other query parameters ( eg compatibible profiles only
func (metaSvc *MetaService) GetProfileMetaDataSummary(profileNames []string) ([]dto2.ProfileSummary, hedgeErrors.HedgeError) {
	metaSvc.service.LoggingClient().Debugf("Getting metadata summary for profiles: %v", profileNames)

	errorMessage := fmt.Sprintf("Error getting metadata summary for profiles: %v", profileNames)

	// For each profile, get contextual data and device attributes via 2 separate API calls
	metaData := make([]dto2.ProfileSummary, 0)
	for _, profile := range profileNames {
		pmetaData := dto2.ProfileSummary{
			Name: profile,
			// Need to add description & any other fields that might be required from profile query later
		}

		//Get the metrics first
		metrics, err := metaSvc.GetMetricsForProfile(profile)
		if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return nil, err
		}
		pmetaData.MetricNames = metrics

		profileContextualAttributes, err := metaSvc.GetProfileContextualAttributes(profile)
		if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return nil, err
		}
		pmetaData.ContextualAttributes = profileContextualAttributes

		deviceExts, err := metaSvc.GetDeviceExtensionInProfile(profile)
		if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return nil, err
		}
		deviceAttributes := make([]string, 0)
		if deviceExts != nil {
			for _, devExt := range deviceExts {
				deviceAttributes = append(deviceAttributes, devExt.Field)
			}
		}

		deviceAttributes = append(deviceAttributes, "host")
		deviceAttributes = append(deviceAttributes, "devicelabels")

		// if there is only one profile, allow deviceName also as a filter criteria
		if len(profileNames) == 1 {
			deviceAttributes = append(deviceAttributes, "deviceName")
		}

		pmetaData.DeviceAttributes = deviceAttributes

		metaData = append(metaData, pmetaData)
	}
	return metaData, nil
}

func (metaSvc *MetaService) GetRelatedProfiles(profileNames []string) ([]string, hedgeErrors.HedgeError) {
	metaSvc.service.LoggingClient().Debugf("Getting related profiles: %v", profileNames)

	errorMessage := fmt.Sprintf("Error getting related profiles for %v", profileNames)

	// Get attributes of the profile first, then get profiles for those attributes
	attributes := make([]string, 0)
	var err hedgeErrors.HedgeError
	for _, profileName := range profileNames {
		contextualAttrs, err := metaSvc.dbClient.GetProfileContextualAttributes(profileName)
		if err == nil || err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			for _, attr := range contextualAttrs {
				if !Contains(attributes, attr) {
					attributes = append(attributes, attr)
				}
			}
		}
		// Now get deviceAttributes as well
		deviceExts, err := metaSvc.dbClient.GetDeviceExtensionInProfile(profileName)
		if err == nil {
			for _, profileExt := range deviceExts {
				if !Contains(attributes, profileExt.Field) {
					attributes = append(attributes, profileExt.Field)
				}
			}
		}
	}

	profiles, err := metaSvc.dbClient.GetProfilesByAttribute(attributes)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return nil, err
	}
	if len(profiles) == 0 && len(profileNames) == 1 {
		profiles = []string{profileNames[0]}
	}

	return profiles, nil
}

func (metaSvc *MetaService) GetAttributesGroupedByProfiles(profileNames []string) (map[string][]string, hedgeErrors.HedgeError) {
	metaSvc.service.LoggingClient().Infof("Getting attributes grouped by profiles: %v", profileNames)

	attributesByProfileColumns := make(map[string][]string)
	attributesByProfileColumns["profileName"] = make([]string, len(profileNames))

	for i, profileName := range profileNames {
		attributesByProfileColumns["profileName"][i] = profileName
		// For the profileName column, add the attribute
		profileSummary, err := metaSvc.GetProfileMetaDataSummary([]string{profileName})
		if err != nil {
			continue
		}
		// Create a combined array of device attributes and contextual attributes
		attrs := append(profileSummary[0].DeviceAttributes, profileSummary[0].ContextualAttributes...)
		for _, attr := range attrs {
			if attr == "deviceName" {
				// devcieName although a device attribute cannot be used to merge a group of devices themselves
				continue
			}
			if _, ok := attributesByProfileColumns[attr]; !ok {
				attributesByProfileColumns[attr] = make([]string, len(profileNames))
			}
			attributesByProfileColumns[attr][i] = attr
		}
	}

	//Filter out and remove if same attr is not present in more than 1 profile
	deleteCandidates := make([]string, 0)
	for attrName, validAttrs := range attributesByProfileColumns {
		validAttrCount := 0
		for _, attrName := range validAttrs {
			if attrName != "" {
				validAttrCount++
			}
		}
		if validAttrCount < 2 {
			deleteCandidates = append(deleteCandidates, attrName)
		}
	}

	for _, attrNameTobeDeleted := range deleteCandidates {
		delete(attributesByProfileColumns, attrNameTobeDeleted)
	}

	return attributesByProfileColumns, nil
}

func (metaSvc *MetaService) GetSQLMetaData(query string) (dto2.SQLMetaData, hedgeErrors.HedgeError) {
	metaSvc.service.LoggingClient().Infof("Getting SQL metadata query: %v", query)
	resp := dto2.SQLMetaData{ProfileName: "", ProfileAlias: "", DeviceAlias: "", MetricNames: []string{}}

	query = strings.ReplaceAll(query, "\\n", "")
	query = strings.ReplaceAll(query, "\\t", "")
	query = strings.ReplaceAll(query, "\n", " ")
	//isolate attributes from query
	attrsArray := regexp2FindAllString(metaSvc.attrPattern, query)
	if len(attrsArray) == 0 {
		metaSvc.service.LoggingClient().Errorf("unable to parse attributes from SQL")
		return resp, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "unable to parse attributes from SQL")
	}
	attrs := attrsArray[0]
	//parse profile name alias
	profAliasArray := regexp2FindAllString(metaSvc.profileAliasPattern, attrs)
	if len(profAliasArray) == 0 {
		resp.ProfileAlias = ""
		metaSvc.service.LoggingClient().Debugf("Unable to parse profile alias from SQL")
	} else {
		resp.ProfileAlias = profAliasArray[0]
	}

	if len(resp.ProfileAlias) != 0 {
		reg := "(?<=" + regParenthesis(resp.ProfileAlias) + "\\s*=\\s*\")[^\"]+(?=\")"

		metaSvc.profileNamePattern = regexp2.MustCompile(reg, regexp2.IgnoreCase)
		profNameArray := regexp2FindAllString(metaSvc.profileNamePattern, query)
		if len(profNameArray) == 0 {
			resp.ProfileName = ""
			metaSvc.service.LoggingClient().Debugf(reg)
			metaSvc.service.LoggingClient().Debugf("Unable to parse profile name from SQL")
		} else {
			resp.ProfileName = profNameArray[0]
		}
	}

	deviceAliasArray := regexp2FindAllString(metaSvc.deviceAliasPattern, attrs)
	if len(deviceAliasArray) == 0 {
		resp.DeviceAlias = ""
		metaSvc.service.LoggingClient().Debugf("Unable to parse device alias from SQL")
	} else {
		resp.DeviceAlias = deviceAliasArray[0]
	}

	//parse metrics (metric without alias or metric with alias or lone metric at beginning or lone metric at end or middle in that order)
	metricArray := regexp2FindAllString(metaSvc.metricPattern, attrs)
	if len(metricArray) != 0 {
		resp.MetricNames = metricArray
	} else {
		resp.MetricNames = []string{}
	}

	return resp, nil
}

func regParenthesis(reg string) string {
	reg = strings.ReplaceAll(reg, "(", "\\(")
	reg = strings.ReplaceAll(reg, ")", "\\)")
	return reg
}

func regexp2FindAllString(re *regexp2.Regexp, s string) []string {
	var matches []string
	m, _ := re.FindStringMatch(s)
	for m != nil {
		matches = append(matches, m.String())
		m, _ = re.FindNextMatch(m)
	}
	return matches
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (metaSvc *MetaService) UpsertDownsamplingConfig(profileName string, downsamplingConfig *dto2.DownsamplingConfig) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("DownsamplingConfig to be added/updated for profile: %s", profileName)

	errorMessage := fmt.Sprintf("Error upserting downsampling config for profile: %s", profileName)

	err := metaSvc.checkProfileExists(errorMessage, profileName)
	if err != nil {
		return err
	}

	err = metaSvc.dbClient.UpsertDownsamplingConfig(profileName, downsamplingConfig)
	if err != nil {
		metaSvc.service.LoggingClient().Warn(err.Error())
	}
	return err
}

func (metaSvc *MetaService) GetDownsamplingConfig(profileName string) (*dto2.DownsamplingConfig, hedgeErrors.HedgeError) {
	var downsamplingConfig *dto2.DownsamplingConfig
	metaSvc.service.LoggingClient().Debugf("Getting Profile DownsamplingConfig for profile: %s", profileName)

	errorMessage := fmt.Sprintf("Error getting downsampling config for profile: %s", profileName)

	exists, _, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		return downsamplingConfig, err
	} else if !exists {
		return downsamplingConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile Not Found", errorMessage))
	}

	downsamplingConfig, err = metaSvc.dbClient.GetDownsamplingConfig(profileName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return downsamplingConfig, err
	}

	return downsamplingConfig, nil
}

func (metaSvc *MetaService) DeleteDownsamplingConfig(profileName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting downsampling config for profile %s", profileName)

	exists, _, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else if !exists {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile %s not found", errorMessage, profileName))
	}

	_, err = metaSvc.dbClient.GetDownsamplingConfig(profileName)
	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Warnf("Profile DownsamplingConfig does not exist for %s. Nothing to delete", profileName)
			return nil
		} else {
			metaSvc.service.LoggingClient().Errorf("%s:%v", errorMessage, err)
			return err
		}
	}

	metaSvc.service.LoggingClient().Infof("Profile DownsamplingConfig to be deleted: %s", profileName)

	err = metaSvc.dbClient.DeleteDownsamplingConfig(profileName)
	return err
}

func (metaSvc *MetaService) UpsertAggregateDefinition(profileName string, downsamplingConfig dto2.DownsamplingConfig) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("Upserting aggregation definition: %s", profileName)

	errorMessage := fmt.Sprintf("Error upserting aggregation definition: %s", profileName)

	exists, _, err := util.CheckProfileExists(metaSvc.MetadataUrl, profileName, metaSvc.service.LoggingClient())
	if err != nil {
		return err
	} else if !exists {
		metaSvc.service.LoggingClient().Errorf("%s: Profile not found", errorMessage)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: Profile not found", errorMessage))
	}

	existingDownsamplingConfig, err := metaSvc.dbClient.GetDownsamplingConfig(profileName)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}
	// merge new Metrics with the existing ones
	for _, newAgg := range downsamplingConfig.Aggregates {
		updateOrAddAggregate(newAgg, &existingDownsamplingConfig.Aggregates)
	}

	err = metaSvc.dbClient.UpsertDownsamplingConfig(profileName, existingDownsamplingConfig)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err
	}

	return nil
}

func (metaSvc *MetaService) UpsertNodeRawDataConfigs(nodeRawDataConfigs []dto2.NodeRawDataConfig) hedgeErrors.HedgeError {
	metaSvc.service.LoggingClient().Infof("NodeRawDataConfigs to be added/updated: %v", nodeRawDataConfigs)

	existingConfigsToReplace := make([]dto2.NodeRawDataConfig, len(nodeRawDataConfigs))
	existingAndNewConfigs := make([]dto2.NodeRawDataConfig, len(nodeRawDataConfigs))
	for i, newConfig := range nodeRawDataConfigs {
		existingConfig, err := metaSvc.dbClient.GetNodeRawDataConfigByNodeID(newConfig.Node.NodeID)
		if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			metaSvc.service.LoggingClient().Errorf("error getting NodeRawDataConfigs from Database for node: %s, error: %s", newConfig.Node.NodeID, err)
			return err
		}
		if existingConfig != nil && err == nil {
			// update existing config
			if !newConfig.SendRawData {
				newConfig.StartTime = existingConfig.StartTime
			}
		}
		existingConfig = &newConfig
		existingConfigsToReplace[i] = *existingConfig
		existingAndNewConfigs[i] = *existingConfig
	}

	err := metaSvc.dbClient.UpsertNodeRawDataConfigs(existingAndNewConfigs, existingConfigsToReplace)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("error updating rawDataConfig: error:%v, newData: %v", err, existingAndNewConfigs)
		return err
	}

	return nil
}

func (metaSvc *MetaService) GetNodeRawDataConfigs(nodeIDs []string) (map[string]*dto2.NodeRawDataConfig, hedgeErrors.HedgeError) {
	nodeRawDataConfigsMap := make(map[string]*dto2.NodeRawDataConfig)

	if len(nodeIDs) == 1 && nodeIDs[0] == "all" {
		metaSvc.service.LoggingClient().Debug("Getting all NodeRawDataConfigs from Database")
		allNodes, err := metaSvc.GetAllNodes()
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("error getting all nodes from hedge-admin service, error: %s", err)
			return nil, err
		}
		for _, node := range allNodes {
			nodeRawDataConfig, err := metaSvc.dbClient.GetNodeRawDataConfigByNodeID(node.NodeId)
			if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Errorf("error getting node raw data config: nodes %v error: %v", nodeIDs, err)
				return nil, err
			}
			if nodeRawDataConfig != nil {
				err = metaSvc.updateExpiredConfig(nodeRawDataConfig)
				if err != nil {
					metaSvc.service.LoggingClient().Errorf("error getting node raw data config: nodes %v error: %v", nodeIDs, err)
					return nil, err
				}
			}
			if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				// In case config is not existing yet for the node - create the default one:
				defaultNodeRawDataConfig := dto2.NodeRawDataConfig{
					SendRawData: false,
					StartTime:   time.Now().Unix(),
					EndTime:     time.Now().Unix(),
					Node: dto2.Node{
						NodeID: node.NodeId,
						Host:   node.HostName,
					},
				}
				err := metaSvc.dbClient.UpsertNodeRawDataConfigs([]dto2.NodeRawDataConfig{defaultNodeRawDataConfig}, []dto2.NodeRawDataConfig{defaultNodeRawDataConfig})
				if err == nil {
					nodeRawDataConfigsMap[node.NodeId] = &defaultNodeRawDataConfig
				}
			} else if nodeRawDataConfig != nil && node.HostName != nodeRawDataConfig.Node.Host {
				nodeRawDataConfig.Node.Host = node.HostName
				err = metaSvc.dbClient.UpsertNodeRawDataConfigs([]dto2.NodeRawDataConfig{*nodeRawDataConfig}, []dto2.NodeRawDataConfig{*nodeRawDataConfig})
				if err == nil {
					nodeRawDataConfigsMap[node.NodeId] = nodeRawDataConfig
				}
			} else {
				nodeRawDataConfigsMap[node.NodeId] = nodeRawDataConfig
			}
		}
	} else {
		for _, nodeID := range nodeIDs {
			metaSvc.service.LoggingClient().Debugf("Getting NodeRawDataConfigs for nodeIDs: %s", nodeIDs)
			nodeRawDataConfig, err := metaSvc.dbClient.GetNodeRawDataConfigByNodeID(nodeID)
			if err != nil && !err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				metaSvc.service.LoggingClient().Errorf("error getting node raw data config: nodes %v error: %v", nodeIDs, err)
				return nil, err
			}
			if nodeRawDataConfig != nil {
				err = metaSvc.updateExpiredConfig(nodeRawDataConfig)
				if err != nil {
					metaSvc.service.LoggingClient().Errorf("error getting node raw data config: nodes %v error: %v", nodeIDs, err)
					return nil, err
				}
			}
			nodeRawDataConfigsMap[nodeID] = nodeRawDataConfig
		}
	}
	return nodeRawDataConfigsMap, nil
}

func updateOrAddAggregate(newAgg dto2.Aggregate, list *[]dto2.Aggregate) {
	for i, agg := range *list {
		if agg.FunctionName == newAgg.FunctionName {
			if newAgg.SamplingIntervalSecs == 0 {
				newAgg.SamplingIntervalSecs = (*list)[i].SamplingIntervalSecs
			}
			if newAgg.GroupBy == "" {
				newAgg.GroupBy = (*list)[i].GroupBy
			}
			// If an Aggregate with the same FunctionName exists, replace it
			(*list)[i] = newAgg
			return
		}
	}
	// If not found, append the new Aggregate
	*list = append(*list, newAgg)
}

func (metaSvc *MetaService) updateExpiredConfig(config *dto2.NodeRawDataConfig) hedgeErrors.HedgeError {
	// In case config is expired (current time is behind the EndTime) - set SendRawData = false in db
	currentTime := time.Now()
	endTime := time.Unix(config.EndTime, 0) // Convert EndTime from int64 timestamp to time.Time

	if currentTime.After(endTime) && config.SendRawData {
		metaSvc.service.LoggingClient().Debugf("NodeRawDataConfig for node %s is expired, updating SendRawData to false.", config.Node.NodeID)
		config.SendRawData = false
		err := metaSvc.dbClient.UpsertNodeRawDataConfigs([]dto2.NodeRawDataConfig{*config}, []dto2.NodeRawDataConfig{*config})
		if err != nil {
			metaSvc.service.LoggingClient().Errorf("Failed to update NodeRawDataConfig for node %s, error: %s", config.Node.NodeID, err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Failed to update NodeRawDataConfig for node %s: %v", config.Node.NodeID, err))
		}
	}

	return nil
}

func (metaSvc *MetaService) GetAllNodes() ([]models.Node, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error getting all nodes")

	hedgeAdminUrl := metaSvc.HedgeAdminUrl + "/api/v3/node_mgmt/node/all"

	req, err := http.NewRequest("GET", hedgeAdminUrl, nil)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("error creating get all nodes request for hedge-admin service, error: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("error executing get all nodes request for hedge-admin service, error: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		metaSvc.service.LoggingClient().Errorf("received non-OK HTTP status from hedge-admin service: %v", res.Status)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("error reading response body: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	var nodes []models.Node
	err = json.Unmarshal(body, &nodes)
	if err != nil {
		metaSvc.service.LoggingClient().Errorf("error unmarshaling response to []models.Node: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return nodes, nil
}

func (metaSvc *MetaService) GetDbClient() redis.DeviceExtDBClientInterface {
	return metaSvc.dbClient
}
