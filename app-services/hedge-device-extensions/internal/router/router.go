/*******************************************************************************
 * Copyright 2017 Dell Inc.
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

package router

import (
	"encoding/json"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	bootstrapinterfaces "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	commonEdgex "github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/app-services/metadata-notifier/pkg"
	"hedge/common/client"
	"hedge/common/config"
	commonconfig "hedge/common/config"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"hedge/app-services/hedge-device-extensions/internal/util"
	"hedge/common/models"
	comService "hedge/common/service"
)

const (
	labelName   = "label"
	profileName = "profile"
	deviceName  = "device"
	serviceName = "service"
	locationId  = "id"
)

type Router struct {
	service            interfaces.ApplicationService
	dbConfig           *db.DatabaseConfig
	metaDataServiceURL string
	metaService        service.MetaDataService
	currentNodeId      string
	telemetry          *Telemetry
	deviceInfoService  *comService.DeviceInfoService
}

func NewRouter(s interfaces.ApplicationService, serviceName string, metricsManager bootstrapinterfaces.MetricsManager) *Router {
	router := new(Router)
	router.service = s

	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(s)
	router.dbConfig = dbConfig

	metadataServiceUrl := getMetadataServiceUrl(s)
	metasyncurl := getMetasyncServiceUrl(s)
	hedgeAdminUrl := getHedgeAdminServiceUrl(s)
	router.currentNodeId, _ = config.GetCurrentNodeIdAndHost(s)
	if router.currentNodeId == "" || metadataServiceUrl == "" || metasyncurl == "" {
		s.LoggingClient().Error("one of metadataServiceURL, metaSyncURL or nodeId is empty in configuration, so exiting")
		os.Exit(-1)
	}

	dbClient := redis.NewDBClient(dbConfig, s.LoggingClient())
	router.metaDataServiceURL = metadataServiceUrl
	router.metaService = service.NewMetaService(s, dbConfig, dbClient, metadataServiceUrl, router.currentNodeId, hedgeAdminUrl)
	router.telemetry = NewTelemetry(s, serviceName, metricsManager, dbClient)
	router.deviceInfoService = comService.GetDeviceInfoService(router.metaDataServiceURL).LoadProfileAndLabels()

	return router
}

func getMetadataServiceUrl(service interfaces.ApplicationService) string {
	metadataServiceUrl, err := service.GetAppSetting("MetaDataServiceUrl")
	if err != nil {
		service.LoggingClient().Errorf("error getting MetaDataServiceUrl configuration :%v", err.Error())
	}
	service.LoggingClient().Infof("MetaDataServiceUrl %v\n", metadataServiceUrl)
	return metadataServiceUrl
}

func getMetasyncServiceUrl(service interfaces.ApplicationService) string {
	metasyncUrl, err := service.GetAppSetting("MetaSyncUrl")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	service.LoggingClient().Infof("MetaSyncUrl %v\n", metasyncUrl)
	return metasyncUrl
}

func getHedgeAdminServiceUrl(service interfaces.ApplicationService) string {
	hedgeAdminUrl, err := service.GetAppSetting("HedgeAdminURL")
	if err != nil {
		service.LoggingClient().Error(err.Error())
	}
	service.LoggingClient().Infof("HedgeAdminURL %v\n", hedgeAdminUrl)
	return hedgeAdminUrl
}

func (r Router) LoadRestRoutes() {
	r.addAssociationRoutes()
	r.addDiscoveryRoutes()
	r.addDeviceInfoRoutes()
	r.addDeviceProfileExtnRoutes()
	r.addDeviceExtnRoutes()
	r.addLocationRoutes()
	r.addProfileContextualData()
	r.addDeviceContextualData()
	r.addProfileAttributes()
	//r.Use(correlation.ManageHeader)
	//r.Use(correlation.OnResponseComplete)
	//r.Use(correlation.OnRequestBegin)
	r.addSQLMetaData()
	r.addDownsamplingRoutes()
	r.addSwaggerRoutes()
}

func (r Router) addAssociationRoutes() {
	r.addRouteRestAddAssociation()

	r.addRouteRestGetAssociation()

	r.addRouteRestUpdateAssociation()

	r.addRouteRestDeleteAssociation()
}

// @Summary		Delete Association For A Given Node
// @Tags	Hedge Device Extensions - Associations
// @Param   	nodeAType     path     string     true  "node A type"
// @Param   	nodeAName     path     string     true  "node A name"
// @Success			200			{string} 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router		/api/v3/metadata/association/{nodeAType}/{nodeAName} [delete]
func (r Router) addRouteRestDeleteAssociation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/association/:nodeAType/:nodeAName", interfaces.Authenticated, func(c echo.Context) error {
		err := restDeleteAssociations(c, r)
		if err != nil {
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.DeleteAction, pkg.NodeType)
		return nil
	}, http.MethodDelete)
}

// @Summary		Update Association For A Given Node
// @Tags	Hedge Device Extensions - Associations
// @Accept		json
// @Produce		json
// @Param   	nodeAType     path     string     true  "node A type"
// @Param   	nodeAName     path     string     true  "node A name"
// @Param 		q 	body 	  []dto.AssociationNode true "list of associated nodes"
// @Success			200			{string} 	id
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router		/api/v3/metadata/association/{nodeAType}/{nodeAName} [put]
func (r Router) addRouteRestUpdateAssociation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/association/:nodeAType/:nodeAName", interfaces.Authenticated, func(c echo.Context) error {
		err := restUpdateAssociations(c, r)
		if err != nil {
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.NodeType)
		return nil
	}, http.MethodPut)
}

// @Summary		Retrieve Association For A Given Node
// @Tags	Hedge Device Extensions - Associations
// @Produce		json
// @Param   	nodeAType     path     string     true  "node A type"
// @Param   	nodeAName     path     string     true  "node A name"
// @Success			200			{array} 	[]dto.AssociationNode
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router		/api/v3/metadata/association/{nodeAType}/{nodeAName} [get]
func (r Router) addRouteRestGetAssociation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/association/:nodeAType/:nodeAName", interfaces.Authenticated, func(c echo.Context) error {
		return restGetAssociations(c, r)
	}, http.MethodGet)
}

// @Summary		Add Association For A Given Node
// @Tags	Hedge Device Extensions - Associations
// @Accept		json
// @Produce		json
// @Param   	nodeAType     path     string     true  "node A type"
// @Param   	nodeAName     path     string     true  "node A name"
// @Param 		q 	body 	  []dto.AssociationNode true "list of associated nodes"
// @Success			200			{string} 	id
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router		/api/v3/metadata/association/{nodeAType}/{nodeAName} [post]
func (r Router) addRouteRestAddAssociation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/association/:nodeAType/:nodeAName", interfaces.Authenticated, func(c echo.Context) error {
		err := restAddAssociations(c, r)
		if err != nil {
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.AddAction, pkg.NodeType)
		return nil
	}, http.MethodPost)
}

func (r Router) addDiscoveryRoutes() {
	// /api/v3/" + DEVICE
	_ = r.service.AddCustomRoute(
		"/device",
		interfaces.Authenticated,
		func(c echo.Context) error {
			// restGetAllDevices(
			// 	w,
			// 	bootstrapContainer.LoggingClientFrom(r.service),
			// 	container.DBClientFrom(dic.Get),
			// 	errorContainer.ErrorHandlerFrom(dic.Get),
			// 	metadataContainer.ConfigurationFrom(dic.Get))
			return nil
		}, http.MethodGet)
	_ = r.service.AddCustomRoute(
		"/GetDiscoverySettings",
		interfaces.Authenticated,
		func(c echo.Context) error {
			// GetDiscoverySettings(
			// 	w,
			// 	bootstrapContainer.LoggingClientFrom(dic.Get),
			// 	container.DBClientFrom(dic.Get),
			// 	errorContainer.ErrorHandlerFrom(dic.Get),
			// 	metadataContainer.ConfigurationFrom(dic.Get))
			return nil
		}, http.MethodGet)
	_ = r.service.AddCustomRoute(
		"/GetDiscoverySettings",
		interfaces.Authenticated,
		func(c echo.Context) error {
			// SaveDiscoverySettings(
			// 	w,
			// 	bootstrapContainer.LoggingClientFrom(dic.Get),
			// 	container.DBClientFrom(dic.Get),
			// 	errorContainer.ErrorHandlerFrom(dic.Get),
			// 	metadataContainer.ConfigurationFrom(dic.Get))
			return nil
		}, http.MethodPost)
	_ = r.service.AddCustomRoute(
		"/ServiceMetaData",
		interfaces.Authenticated,
		func(c echo.Context) error {
			// GetServiceMetaData(
			// 	w,
			// 	bootstrapContainer.LoggingClientFrom(dic.Get),
			// 	container.DBClientFrom(dic.Get),
			// 	errorContainer.ErrorHandlerFrom(dic.Get),
			// 	metadataContainer.ConfigurationFrom(dic.Get))
			return nil
		}, http.MethodPost)
}

func (r Router) addDeviceInfoRoutes() {
	r.addRouteDeleteRestDeviceInfo()

	r.addRouteRestCreateCompleteProfile()

	r.addRouteRestUpdateCompleteProfile()

	r.addRouteRestGetCompleteProfile()

	r.addRouteRestDeleteCompleteProfile()

	r.addRouteRestGetProfileTags()

	r.addRouteRestGetProfilesByTags()

	r.addRouteRestCreateCompleteDevice()

	r.addRouteRestUpdateCompleteDevice()

	r.addRouteRestGetCompleteDevice()

	r.addRouteRestDeleteCompleteDevice()

	r.addRouteGetGivenDevicesSummary()

	r.addRouteGetLabels()

	// New wrapper around edgex metadata getProfilesAll added to have the current UI tabular list working in v2
	r.addRouteGetProfiles()

	r.addRouteGetProfileNames()

	r.addRouteGetDevicesByProfile()

	r.addRouteGetDevicesByLabel()

	r.addRouteGetMetricsForProfile()

	r.addRouteGetMetricsByDevices()

	r.addRouteGetDeviceServices()

	r.addRouteGetDeviceServiceByServiceName()

	r.addRouteGetDeviceServiceProtocols()

	r.addRouteGetDevicesSummary()
}

// @Summary Get Devices Summary
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param 			serviceName 	path string true "service name"
// @Success 		200 		{object} 	ResultDeviceSummary "device summary"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/devices [get]
func (r Router) addRouteGetDevicesSummary() {
	_ = r.service.AddCustomRoute("/api/v3/devices", interfaces.Authenticated, func(c echo.Context) error {
		return getDevicesSummary(c, r)
	}, http.MethodGet)
}

// @Summary Get Device Service Protocols
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param 			serviceName 	path string true "service name"
// @Success 		200 		{object} 	models.DeviceServiceProtocols "map of device service protocols"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/deviceservice/{serviceName}/protocols [get]
func (r Router) addRouteGetDeviceServiceProtocols() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/deviceservice/:"+serviceName+"/protocols", interfaces.Authenticated, func(c echo.Context) error {
		return getDeviceServiceProtocols(c, r)
	}, http.MethodGet)
}

// @Summary Get Device Services by Service Name
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param 			serviceName 	path string true "service name"
// @Success 		200 		{object} 	interface{} "List of device services"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/deviceservice/name/{serviceName} [get]
func (r Router) addRouteGetDeviceServiceByServiceName() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/deviceservice/name/:"+serviceName, interfaces.Authenticated, func(c echo.Context) error {
		svcName := c.Param(serviceName)
		services, err := getDeviceService(c, r, svcName)

		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}

		return c.JSON(http.StatusOK, services)
	}, http.MethodGet)
}

// @Summary Get Device Services
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param 			edgeNode 	path string true "edgeNode"
// @Success 		200 		{array} 	string "List of device services"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/deviceservice [get]
func (r Router) addRouteGetDeviceServices() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/deviceservice", interfaces.Authenticated, func(c echo.Context) error {
		nodeId := c.QueryParam("edgeNode")
		services, err := getDeviceServices(r, nodeId)

		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}

		return c.JSON(http.StatusOK, services)
	}, http.MethodGet)
}

// @Summary Get Metrics by Devices
// @Tags	Hedge Device Extensions - Metrics
// @Produce json
// @Param devices path string true "list of device names"
// @Success 		200 		{array} 	string "Metrics data for the specified profile"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/metrics [get]
func (r Router) addRouteGetMetricsByDevices() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/metrics", interfaces.Authenticated, func(c echo.Context) error {

		deviceNames := c.QueryParam("devices")
		devices := strings.Split(deviceNames, ",")
		body := r.deviceInfoService.GetMetricsByDevices(devices)

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary Get Metrics for Profile
// @Description Retrieves metrics data associated with the specified device profile.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param profileName path string true "Name of the device profile"
// @Success 		200 		{array} 	string "Metrics data for the specified profile"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/profile/{profileName}/metrics [get]
func (r Router) addRouteGetMetricsForProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/:"+profileName+"/metrics", interfaces.Authenticated, func(c echo.Context) error {

		profile := c.Param(profileName)
		metrics, err := r.metaService.GetMetricsForProfile(profile)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		return c.JSON(http.StatusOK, metrics)
	}, http.MethodGet)
}

// @Summary Retrieve Devices by Label
// @Description Retrieves a list of devices that match the specified labels. If the `curDevice` query parameter is provided, devices associated with the current device's labels are excluded.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param labelName path string true "Comma-separated list of labels to filter devices by"
// @Param curDevice query string false "Current device ID to exclude devices associated with its labels"
// @Success 		200 		{array} 	string "List of device names matching the labels"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/label/{labelName}/devices [get]
func (r Router) addRouteGetDevicesByLabel() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/label/:"+labelName+"/devices", interfaces.Authenticated, func(c echo.Context) error {

		label := c.Param(labelName)
		labels := strings.Split(label, ",")

		body := r.deviceInfoService.GetDevicesByLabelsCriteriaOR(labels)

		curDevice, exists := c.Request().URL.Query()["curDevice"]
		var result []string
		if exists {
			curDevLabels, err := r.metaService.GetAssociation(curDevice[0])
			curDevLabelsMap := make(map[string]bool)
			for _, name := range curDevLabels {
				curDevLabelsMap[name.NodeName] = true
			}
			if err != nil {
				r.service.LoggingClient().Error(err.Error())
				return err.ConvertToHTTPError()
			}
			for _, device := range body {
				_, found := curDevLabelsMap[device]
				if found || device == curDevice[0] {
					continue
				} else {
					result = append(result, device)
				}
			}
		} else {
			result = body
		}

		return c.JSON(http.StatusOK, result)
	}, http.MethodGet)
}

// @Summary Get Devices by Profile
// @Description Retrieves a list of devices that are associated with the specified device profile.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param profileName path string true "Profile Name"
// @Success 		200 		{array} []string "List of devices associated with the profile"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/profile/{profileName}/devices [get]
func (r Router) addRouteGetDevicesByProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/:"+profileName+"/devices", interfaces.Authenticated, func(c echo.Context) error {

		profile := c.Param(profileName)

		body := r.deviceInfoService.GetDevicesByProfile(profile)
		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary Get Device Profile Names
// @Description Retrieves a list of all device profile names from the metadata service.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Success 		200 		{array} string "List of device profile names"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/profile [get]
func (r Router) addRouteGetProfileNames() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile", interfaces.Authenticated, func(c echo.Context) error {

		body, err := util.GetProfileNames(r.metaDataServiceURL, r.service.LoggingClient())
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary Get Device Profiles
// @Description Retrieves a list of all device profiles from the metadata service.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Success 200 {array} []interface{} "List of device profiles"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/deviceprofile [get]
func (r Router) addRouteGetProfiles() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/deviceprofile", interfaces.Authenticated, func(c echo.Context) error {
		body, err := util.GetProfiles(r.metaDataServiceURL, r.service.LoggingClient())
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary Get Device Labels
// @Description Retrieves a list of all available device labels.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Success 200 {array} string "List of device labels"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/label [get]
func (r Router) addRouteGetLabels() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/label", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debug("Get Device Labels")

		body := r.deviceInfoService.GetLabels()

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary Get Devices Information Summary
// @Description Retrieves a summary of the specified devices based on the provided names.
// @Tags	Hedge Device Extensions - Devices
// @Accept json
// @Produce json
// @Param 	q body []string true "List of the names of the devices"
// @Success 		200 		{array} 	[]dtos.Device "List of devices full infos"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/devices [post]
func (r Router) addRouteGetGivenDevicesSummary() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/devices", interfaces.Authenticated, func(c echo.Context) error {
		return getGivenDevicesSummary(c, r)
	}, http.MethodPost)
}

// @Summary Delete Complete Device
// @Description Deletes the specified device
// @Tags	Hedge Device Extensions - Devices
// @Param 		deviceName path string true "Device Name"
// @Success			200			{string}	OK 	"OK"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/device/name/{deviceName} [delete]
func (r Router) addRouteRestDeleteCompleteDevice() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/device/name/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return restDeleteCompleteDevice(c, r)
	}, http.MethodDelete)
}

// @Summary Get Complete Device Info
// @Description Retrieves detailed information about a device by its name.
// @Tags	Hedge Device Extensions - Devices
// @Produce json
// @Param		deviceName	path		string				true	"device name"
// @Success			200			{object}	dto.DeviceObject
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router /api/v3/deviceinfo/device/name/{deviceName} [get]
func (r Router) addRouteRestGetCompleteDevice() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/device/name/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return restGetCompleteDevice(c, r)
	}, http.MethodGet)
}

// @Summary		Update Complete Device
// @Description	Updating device
// @Tags	Hedge Device Extensions - Devices
// @Accept		json
// @Produce		json
// @Param   	deviceName     path     string     false  "deviceName"
// @Success		200			{object} 	ResultId
// @Failure		400			{object}	error	"{"message":"Error message"}"
// @Failure		500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/device/name/{deviceName} [put]
func (r Router) addRouteRestUpdateCompleteDevice() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/device/name/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return restUpdateCompleteDevice(c, r)
	}, http.MethodPut)
}

// @Summary		Create Complete Device
// @Description	Creating complete device
// @Tags	Hedge Device Extensions - Devices
// @Produce		json
// @Accept		json
// @Param request body dto.DeviceObject true "query params"
// @Success		200			{object}	ResultId
// @Failure		400			{object}	error	"{"message":"Error message"}"
// @Failure		500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/device/name/{deviceName} [post]
func (r Router) addRouteRestCreateCompleteDevice() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/device/name/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return restCreateCompleteDevice(c, r)
	}, http.MethodPost)
}

// @Summary		Get profiles by tags
// @Description	Retrieving profile by tags
// @Tags	Hedge Device Extensions - Profiles
// @Produce		json
// @Param   	collection  query     []string   true  "string collection"  collectionFormat(multi)
// @Success		200			{array}	dto.ProfileObject
// @Failure		400			{object}	error	"{"message":"Error message"}"
// @Failure		500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/tags/profiles [get]
func (r Router) addRouteRestGetProfilesByTags() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/tags/profiles", interfaces.Authenticated, func(c echo.Context) error {
		return restGetProfilesByTags(c, r)
	}, http.MethodGet)
}

// @Summary		Get profile tags
// @Description	Retrieving profile tags
// @Tags	Hedge Device Extensions - Profiles
// @Accept		json
// @Produce		json
// @Param		profileName	path		string				true	"profile name"
// @Success			200			{array}	string
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/tags [get]
func (r Router) addRouteRestGetProfileTags() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/tags", interfaces.Authenticated, func(c echo.Context) error {
		return restGetProfileTags(c, r)
	}, http.MethodGet)
}

// @Summary		Delete Profile
// @Description	Deleting profile
// @Tags	Hedge Device Extensions - Profiles
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}	dto.ProfileObject
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/name/{profileName} [delete]
func (r Router) addRouteRestDeleteCompleteProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/name/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return restDeleteCompleteProfile(c, r)
	}, http.MethodDelete)
}

// @Summary		Get Complete Profile
// @Description	Retrieving profile
// @Tags	Hedge Device Extensions - Profiles
// @Accept			json
// @Produce			json
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}	dto.ProfileObject
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/name/{profileName} [get]
func (r Router) addRouteRestGetCompleteProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/name/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return restGetCompleteProfile(c, r)
	}, http.MethodGet)
}

// @Summary		Update Profile
// @Description	Updating existing profile
// @Tags	Hedge Device Extensions - Profiles
// @Accept			json
// @Produce			json
// @Param			q			body		dto.ProfileObject	true	"Profile Object"
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}	ResultId
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/name/{profileName} [put]
func (r Router) addRouteRestUpdateCompleteProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/name/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return restUpdateCompleteProfile(c, r)
	}, http.MethodPut)
}

// @Summary		Create Complete Profile
// @Description	Profile creation
// @Tags	Hedge Device Extensions - Profiles
// @Accept			json
// @Produce			json
// @Param			q			body		dto.ProfileObject	true	"Profile Object"
// @Param			profileName	path		string				true	"profile name"
// @Success		200			{string}	OK 	"OK"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo/profile/name/{profileName} [post]
func (r Router) addRouteRestCreateCompleteProfile() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo/profile/name/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return restCreateCompleteProfile(c, r)
	}, http.MethodPost)
}

// @Summary		Clearing cache of device service
// @Description	Clears cache of DeviceInfoService
// @Tags	Hedge Device Extensions - Devices
// @Accept			json
// @Produce			json
// @Success			200			{object}	string 	"{"message": "Cache cleared successfully"}"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/deviceinfo [delete]
func (r Router) addRouteDeleteRestDeviceInfo() {
	_ = r.service.AddCustomRoute("/api/v3/deviceinfo", interfaces.Authenticated, func(c echo.Context) error {
		r.deviceInfoService.ClearCache()
		return c.JSON(http.StatusOK, map[string]string{"message": "Cache cleared successfully"})
	}, http.MethodDelete)
}

func (r Router) addLocationRoutes() {

	r.addRouteRestAddLocation()

	r.addRouteRestGetLocation()

	r.addRouteRestFindLocation()
}

// @Summary		Find Location by Id
// @Tags	Hedge Device Extensions - Location
// @Produce			json
// @Param			country	query		string				true	"country"
// @Param			state	query		string				true	"state"
// @Param			city	query		string				true	"city"
// @Success			200			{object}	models.Location 	"Location object"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/location/{locationId}	[get]
func (r Router) addRouteRestFindLocation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/location", interfaces.Authenticated, func(c echo.Context) error {
		// Have query parameter : labels = label1,label2,..
		// To get devices by profile, query param will be profile=profile1
		country := c.QueryParam("country")
		state := c.QueryParam("state")
		city := c.QueryParam("city")

		body, err := r.metaService.GetLocations(country, state, city)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary		Get Location by Id
// @Tags	Hedge Device Extensions - Location
// @Produce			json
// @Param			locationId	path		string				true	"location id"
// @Success			200			{object}	models.Location 	"Location object"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/location/{locationId}	[get]
func (r Router) addRouteRestGetLocation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/location/:"+locationId, interfaces.Authenticated, func(c echo.Context) error {
		// Have query parameter : labels = label1,label2,..
		// To get devices by profile, query param will be profile=profile1

		id := c.Param(locationId)
		body, err := r.metaService.GetLocation(id)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		return c.JSON(http.StatusOK, body)
	}, http.MethodGet)
}

// @Summary		Add Location
// @Tags	Hedge Device Extensions - Location
// @Accept			json
// @Produce			json
// @Param 			q 	body 	models.Location 	true 	"location"
// @Success			200			{object}	map[string]string "map of locations"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/location	[post]
func (r Router) addRouteRestAddLocation() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/location", interfaces.Authenticated, func(c echo.Context) error {
		// Have query parameter : labels = label1,label2,..
		// To get devices by profile, query param will be profile=profile1

		var location models.Location
		err := json.NewDecoder(c.Request().Body).Decode(&location)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}

		body, err := r.metaService.AddLocation(location)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}

		return c.JSON(http.StatusOK, body)
	}, http.MethodPost)
}

func (r Router) addDeviceProfileExtnRoutes() {
	r.addRouteRestAddDeviceProfileExt()

	r.addRouteRestUpdateDeviceProfileExt()

	r.addRouteRestGetDeviceProfileExt()

	r.addRouteRestDeleteDeviceProfileExt()
}

// @Summary		Delete Device Profile Extensions
// @Tags	Hedge Device Extensions - Device Profile Extensions
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}	string 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/extn/{profileName} [delete]
func (r Router) addRouteRestDeleteDeviceProfileExt() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/extn/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		err := restDeleteDvcProfileExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.DeleteAction, pkg.ProfileType)
		return nil
	}, http.MethodDelete)
}

// @Summary		Retrieve Device Profile Extensions
// @Tags	Hedge Device Extensions - Device Profile Extensions
// @Produce	json
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{array}	[]dto.DeviceExtension 	"will bring empty array in case no extensions found"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/extn/{profileName} [get]
func (r Router) addRouteRestGetDeviceProfileExt() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/extn/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return restGetDvcProfileExt(c, r)
	}, http.MethodGet)
}

// @Summary		Update Device Profile Extensions
// @Tags	Hedge Device Extensions - Device Profile Extensions
// @Produce	json
// @Accept	json
// @Param			profileName	path		string				true	"profile name"
// @Param			q			body		[]dto.DeviceExtension	true	"array of device extensions object"
// @Success			200			{object}	ResultId 	""
// @Success			200			{array}		[]string 	"empty array"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/extn/{profileName} [put]
func (r Router) addRouteRestUpdateDeviceProfileExt() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/extn/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		err := restUpdateDvcProfileExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.ProfileType)
		return nil
	}, http.MethodPut)
}

// @Summary		Create Device Profile Extensions
// @Tags	Hedge Device Extensions - Device Profile Extensions
// @Produce	json
// @Accept	json
// @Param			profileName	path		string				true	"profile name"
// @Param			q			body		[]dto.DeviceExtension	true	"array of device extensions object"
// @Success			200			{object}	ResultId 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/extn/{profileName} [post]
func (r Router) addRouteRestAddDeviceProfileExt() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/extn/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		err := restAddDvcProfileExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.AddAction, pkg.ProfileType)
		return nil
	}, http.MethodPost)
}

func (r Router) addDeviceExtnRoutes() {
	r.addRouteRestAddDeviceExtn()

	r.addRouteRestUpdateDeviceExtn()

	r.addRouteRestGetDeviceExtn()

	r.addRouteRestDeleteDeviceExtn()
}

// @Summary		Delete Device Extensions
// @Tags	Hedge Device Extensions - Device Extensions
// @Param			deviceName	path		string				true	"device name"
// @Success			200			{string}	string 	"no content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/extn/{deviceName} [delete]
func (r Router) addRouteRestDeleteDeviceExtn() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/extn/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		err := restDeleteAllDeviceExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.DeleteAction, pkg.DeviceType)
		return nil
	}, http.MethodDelete)
}

// @Summary		Retrieve Device Extensions
// @Tags	Hedge Device Extensions - Device Extensions
// @Produce	json
// @Param			deviceName	path		string				true	"device name"
// @Success			200			{array}		[]dto.DeviceExtension 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/extn/{deviceName} [get]
func (r Router) addRouteRestGetDeviceExtn() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/extn/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return restGetDeviceExt(c, r)
	}, http.MethodGet)
}

// @Summary		Update Device Extensions
// @Tags	Hedge Device Extensions - Device Extensions
// @Produce	json
// @Accept	json
// @Param			deviceName	path		string				true	"device name"
// @Param 			q 			body 		[]dto.DeviceExt 	true 	"List of the device extensions objects"
// @Success			200			{array}		[]dto.DeviceExtension 	""
// @Success			200			{array}		[]string 	"empty array"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/extn/{deviceName} [put]
func (r Router) addRouteRestUpdateDeviceExtn() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/extn/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		err := restUpdateDeviceExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.DeviceType)
		return nil
	}, http.MethodPut)
}

// @Summary		Create Device Extensions
// @Tags	Hedge Device Extensions - Device Extensions
// @Produce	json
// @Accept	json
// @Param			deviceName	path		string				true	"device name"
// @Param 			q 			body 		[]dto.DeviceExt 	true 	"List of the device extensions objects"
// @Success			200			{object}	ResultId 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/extn/{deviceName} [post]
func (r Router) addRouteRestAddDeviceExtn() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/extn/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		err := restAddDeviceExt(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.AddAction, pkg.DeviceType)
		return nil
	}, http.MethodPost)
}

func (r Router) addProfileContextualData() {
	r.addRouteRestAddProfileContextualData()

	r.addRouteRestGetProfileContextualData()

	r.addRouteRestDeleteProfileContextualData()
}

// @Summary		Delete Profile Contextual Data
// @Tags	Hedge Device Extensions - Profile Contextual Data
// @Produce	json
// @Accept	json
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}		ResultDeviceName 	"Profile name from which contextual data was removed"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/biz/{profileName} [delete]
func (r Router) addRouteRestDeleteProfileContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/biz/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		err := deleteProfileContextualData(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.DeleteAction, pkg.ProfileType)
		return nil
	}, http.MethodDelete)
}

// @Summary		Retrieve Profile Contextual Data
// @Tags	Hedge Device Extensions - Profile Contextual Data
// @Produce	json
// @Accept	json
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{array}		[]string 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/biz/{profileName} [get]
func (r Router) addRouteRestGetProfileContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/biz/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		return getProfileContextualData(c, r)
	}, http.MethodGet)
}

// @Summary		Add Profile Contextual Data
// @Tags	Hedge Device Extensions - Profile Contextual Data
// @Accept	json
// @Param			profileName	path		string				true	"profile name"
// @Param 			q 			body 		[]string 			true 	"List of the contextual attributes"
// @Success			200			{string}	string 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/biz/{profileName} [post]
func (r Router) addRouteRestAddProfileContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/biz/:"+profileName, interfaces.Authenticated, func(c echo.Context) error {
		err := upsertProfileContextualAttributes(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.AddAction, pkg.ProfileType)
		return nil
	}, http.MethodPost)
}

func (r Router) addDeviceContextualData() {
	// Deprecation
	r.service.AddCustomRoute("/api/v3/metadata/device/biz/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		reqBody, err := getRequestBody(c.Response(), c.Request(), r)

		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}

		var existingContextualData map[string]interface{}
		devName := c.Param(deviceName)
		existingContextualData, errC := r.metaService.GetDeviceContextualAttributes(devName)
		if errC != nil && !errC.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().Error(err.Error())
			return errC.ConvertToHTTPError()
		}

		err = upsertDeviceContextualData(c, r, reqBody)
		if err == nil {
			err1 := r.telemetry.ContextualDataRequest(existingContextualData, reqBody)
			if err1 != nil {
				r.service.LoggingClient().Error(err.Error())
				return err1.ConvertToHTTPError()
			}
		}
		// trigger node sync
		r.notifySync(c, pkg.AddAction, pkg.DeviceType)
		return nil
	}, http.MethodPost)

	// Update the implementation to automatically add contextual metadata to corresponding profile
	r.addRouteRestUpdateDeviceContextualData()

	r.addRouteRestGetDeviceContextualData()

	r.addRouteRestDeleteDeviceContextualData()
}

// @Summary		Delete Device Contextual Data
// @Tags	Hedge Device Extensions - Device Contextual Data
// @Produce	json
// @Param			deviceName	path		string				true	"device name"
// @Success			200			{object}	ResultDeviceName 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/biz/{deviceName} [delete]
func (r Router) addRouteRestDeleteDeviceContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/biz/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		err := deleteDeviceContextualData(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.DeleteAction, pkg.DeviceType)
		return nil
	}, http.MethodDelete)
}

// @Summary		Retrieve Device Contextual Data
// @Tags	Hedge Device Extensions - Device Contextual Data
// @Produce	json
// @Param			deviceName	path		string				true	"device name"
// @Success			200			{object}	map[string]interface{} 	"Contextual data map"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/biz/{deviceName} [get]
func (r Router) addRouteRestGetDeviceContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/biz/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		return getDeviceContextualData(c, r)
	}, http.MethodGet)
}

// @Summary		Update Device Contextual Data
// @Description Update the implementation to automatically add contextual metadata to corresponding profile
// @Tags	Hedge Device Extensions - Device Contextual Data
// @Accept	json
// @Param			deviceName	path		string				true	"device name"
// @Param 			q 			body 		map[string]interface{} 			true 	"map of device contextual data"
// @Success			200			{string}	string 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/biz/{deviceName} [patch]
func (r Router) addRouteRestUpdateDeviceContextualData() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/device/biz/:"+deviceName, interfaces.Authenticated, func(c echo.Context) error {
		reqBody, err := getRequestBody(c.Response(), c.Request(), r)
		if err != nil {
			return err
		}
		var existingContextualData map[string]interface{}
		devName := c.Param(deviceName)
		existingContextualData, errC := r.metaService.GetDeviceContextualAttributes(devName)
		if errC != nil && !errC.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().Error(err.Error())
			return errC.ConvertToHTTPError()
		}
		err = upsertDeviceContextualData(c, r, reqBody)
		if err == nil {
			err1 := r.telemetry.ContextualDataRequest(existingContextualData, reqBody)
			if err1 != nil {
				r.service.LoggingClient().Error(err.Error())
				return err1.ConvertToHTTPError()
			}
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.DeviceType)
		return nil
	}, http.MethodPatch)
}

func getRequestBody(writer http.ResponseWriter, req *http.Request, r Router) ([]byte, *echo.HTTPError) {
	reqBody, err := io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		r.service.LoggingClient().Errorf("Error reading the request payload : %v", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	return reqBody, nil
}

// Add routes for the combined device attribute and business attribute
func (r Router) addProfileAttributes() {
	// Add a method to get combined business and extn attribute to ease ML UI devp
	// Have query parameters: profiles=profile1,profile2, all
	// Output is grouped by Profile, then by contextual attr/device attr
	r.addRouteRestGetProfileMetadataSummary()

	// Get related profiles based on 1 or more profiles that is already provided
	r.addRouteRestGetRelatedProfiles()

	// Get attributes grouped by attribute, then by profile1, profile2,...
	r.addRouteRestGetAttributesGroupedByProfiles()
}

// @Summary		Retrieve attributes grouped by profile
// @Description Retrieves combined business and extensions attribute to ease ML UI development. Output is grouped by Profile, then by contextual attr/device attributes
// @Tags	Hedge Device Extensions - Profile Attribute
// @Produce	json
// @Param			profiles	path		string				true	"profiles=profile1,profile2; all"
// @Success			200			{object}	map[string][]string 	"Attributes grouped by profiles, then by contextual/device attributes"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router	/api/v3/metadata/deviceprofile/attributes	[get]
func (r Router) addRouteRestGetAttributesGroupedByProfiles() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/attributes", interfaces.Authenticated, func(c echo.Context) error {
		return getAttributesGroupedByProfiles(c, r)
	}, http.MethodGet)
}

// @Summary		Retrieve related profiles' names
// @Description Retrieves related profiles based on 1 or more profiles that is already provided
// @Tags	Hedge Device Extensions - Profile Attribute
// @Produce	json
// @Param			profiles	path		string				true	"profiles=profile1,profile2"
// @Success			200			{array}	[]string 	"Names of related profiles"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router	/api/v3/metadata/deviceprofile/relatedprofiles	[get]
func (r Router) addRouteRestGetRelatedProfiles() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/relatedprofiles", interfaces.Authenticated, func(c echo.Context) error {
		return getRelatedProfiles(c, r)
	}, http.MethodGet)
}

// @Summary		Retrieve profile metadata summary
// @Description Retrieves attributes grouped by attribute, then by profile1, profile2
// @Tags	Hedge Device Extensions - Profile Attribute
// @Produce	json
// @Param			profiles	path		string				true	"profiles=profile1,profile2"
// @Success			200			{array}		[]dto.ProfileSummary 	"List of ProfileSummary objects"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router	/api/v3/metadata/deviceprofile/summary	[get]
func (r Router) addRouteRestGetProfileMetadataSummary() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/summary", interfaces.Authenticated, func(c echo.Context) error {
		return getProfileMetaDataSummary(c, r)
	}, http.MethodGet)
}

// Add routes to support downsampling
func (r Router) addDownsamplingRoutes() {
	// Add or update Downsampling Config to an existing profile
	// If there is Downsampling Config present, it will replace the existing config during update
	r.addRouteRestUpdateDownsamplingConfig()

	// Get Downsampling Config for a given profileName
	r.addRouteRestGetDownsamplingConfig()

	// Add or update an Aggregate functions
	// Here old data is not deleted, if it exists, we update it, otherwise we delete it
	r.addRouteRestUpdateAggregateFunctionDefinition()

	// Add or update NodeRawDataConfigs
	// If there are NodeRawDataConfigs present, it will replace each existing config in case it was changed
	// And add new one in case it doesn't exist yet
	r.addRouteRestUpdateNodeRawDataConfigs()

	// Get all NodeRawDataConfigs for a given node ids
	r.addRouteRestGetNodeRawDataConfigs()
}

func (r Router) addRouteRestGetNodeRawDataConfigs() {
	_ = r.service.AddCustomRoute("/api/v3/node_mgmt/downsampling/rawdata", interfaces.Authenticated, func(c echo.Context) error {
		return getNodeRawDataConfigs(c, r)
	}, http.MethodGet)
}

// @Summary		Update Node RawData Configuration
// @Description If there are NodeRawDataConfigs present, it will replace each existing config in case it was changed or add new one in case it doesn't exist yet
// @Tags	Hedge Device Extensions - Node Raw Data Configuration
// @Accept	json
// @Param 			q 			body 		[]dto.NodeRawDataConfig	true 	"list of node raw data configurations"
// @Success			200			{string}	string 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/node_mgmt/downsampling/rawdata [put]
func (r Router) addRouteRestUpdateNodeRawDataConfigs() {
	_ = r.service.AddCustomRoute("/api/v3/node_mgmt/downsampling/rawdata", interfaces.Authenticated, func(c echo.Context) error {
		err := upsertNodeRawDataConfigs(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.NodeType)
		return nil
	}, http.MethodPut)
}

// @Summary		Update Aggregate Function Definition
// @Description Adds or updates an Aggregate functions. Here old data is not deleted; if it exists, we update it, otherwise we delete it
// @Tags	Hedge Device Extensions - Downsampling
// @Accept	json
// @Param 			q 			body 		[]dto.DownsamplingConfig	true 	"list of node raw data configurations"
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{string}	string 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/{profileName}/downsampling/config [patch]
func (r Router) addRouteRestUpdateAggregateFunctionDefinition() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/:"+profileName+"/downsampling/config", interfaces.Authenticated, func(c echo.Context) error {
		err := upsertAggregateDefinition(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.ProfileType)
		return nil
	}, http.MethodPatch)
}

// @Summary		Retrieve Downsampling configuration for a given profile name
// @Tags	Hedge Device Extensions - Downsampling
// @Produce	json
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{object}	dto.DownsamplingConfig 	""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/{profileName}/downsampling/config [get]
func (r Router) addRouteRestGetDownsamplingConfig() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/:"+profileName+"/downsampling/config", interfaces.Authenticated, func(c echo.Context) error {
		return getDownsamplingConfig(c, r)
	}, http.MethodGet)
}

// @Summary		Update Downsampling Config for existing profile
// @Description If there is Downsampling Config present, it will replace the existing config during update
// @Tags	Hedge Device Extensions - Downsampling
// @Accept	json
// @Produce	json
// @Param 			q 			body 		[]dto.DownsamplingConfig	true 	"list of node raw data configurations"
// @Param			profileName	path		string				true	"profile name"
// @Success			200			{string}	string 	"No content"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/deviceprofile/{profileName}/downsampling/config [put]
func (r Router) addRouteRestUpdateDownsamplingConfig() {
	_ = r.service.AddCustomRoute("/api/v3/metadata/deviceprofile/:"+profileName+"/downsampling/config", interfaces.Authenticated, func(c echo.Context) error {
		err := upsertDownsamplingConfig(c, r)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		// trigger node sync
		r.notifySync(c, pkg.UpdateAction, pkg.ProfileType)
		return nil
	}, http.MethodPut)
}

func (r Router) addSwaggerRoutes() {
	_ = r.service.AddCustomRoute("swagger.json", interfaces.Authenticated, func(c echo.Context) error {
		swaggerFileContent, err := readSwaggerFile("swagger.json")
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		var spec = make(map[string]interface{})
		if err := json.Unmarshal(swaggerFileContent, &spec); err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "Error reading swagger specification")
		}

		if err := c.JSON(http.StatusOK, spec); err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "Error reading swagger specification")
		}

		return nil
	}, http.MethodGet)
	_ = r.service.AddCustomRoute("/index.html", interfaces.Authenticated, echoSwagger.EchoWrapHandler(echoSwagger.URL("swagger.json")), http.MethodGet)
	_ = r.service.AddCustomRoute("/*", interfaces.Authenticated, func(c echo.Context) error {
		path := c.Request().URL.Path
		file, _ := getSwaggerFilePath(filepath.Join("content", "swagger", filepath.FromSlash(path)))
		_ = c.File(file)
		return nil
	}, http.MethodGet)
}

func (r Router) GetDeviceInfoInstance() *comService.DeviceInfoService {
	return r.deviceInfoService
}

func (r Router) addSQLMetaData() {
	r.service.AddCustomRoute("/api/v3/sql/fields", interfaces.Authenticated, func(c echo.Context) error {
		return getSQLMetaData(c, r)
	}, http.MethodPost)
}

func (r Router) GetDbClient() redis.DeviceExtDBClientInterface {
	return r.metaService.GetDbClient()
}

func (r Router) notifySync(c echo.Context, action, resourceType string) {
	topic, err := r.service.GetAppSetting("SystemPublishTopic")
	if err != nil {
		r.service.LoggingClient().Errorf("failed to get app setting err: %s", err.Error())
		return
	}

	var events []*dtos.SystemEvent
	switch resourceType {
	case pkg.DeviceType:
		e := dtos.SystemEvent{
			Owner:   client.HedgeDeviceExtnsServiceKey,
			Action:  action,
			Type:    resourceType,
			Details: dtos.Device{Name: c.Param(deviceName)}}
		events = append(events, &e)
	case pkg.ProfileType:
		e := dtos.SystemEvent{
			Owner:   client.HedgeDeviceExtnsServiceKey,
			Action:  action,
			Type:    resourceType,
			Details: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: c.Param(profileName)}},
		}
		events = append(events, &e)
	case pkg.NodeType:
		nodes, _ := commonconfig.GetAllNodes(r.service)
		for _, n := range nodes {
			e := dtos.SystemEvent{
				Owner:   client.HedgeDeviceExtnsServiceKey,
				Action:  action,
				Type:    resourceType,
				Details: n.NodeId,
			}
			events = append(events, &e)
		}
	default:
		e := dtos.SystemEvent{
			Owner:  client.HedgeDeviceExtnsServiceKey,
			Action: action,
			Type:   resourceType,
		}
		events = append(events, &e)
	}

	for _, e := range events {
		e := r.service.PublishWithTopic(topic, e, commonEdgex.ContentTypeJSON)
		if e != nil {
			r.service.LoggingClient().Errorf("failed sending system event err: %s", e.Error())
		}
	}
	r.service.LoggingClient().Info("system updated sent system event...")
}

func getSwaggerFilePath(internalPath string) (string, hedgeErrors.HedgeError) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error getting path to swagger specification file")
	}

	return filepath.Join(workDir, filepath.FromSlash(internalPath)), nil
}

func readSwaggerFile(internalPath string) ([]byte, hedgeErrors.HedgeError) {
	filePath, hErr := getSwaggerFilePath(internalPath)
	if hErr != nil {
		return nil, hErr
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Swagger specification file not found")
	}

	return bytes, nil
}
