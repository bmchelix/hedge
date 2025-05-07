/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"hedge/app-services/hedge-admin/internal/config"
	adminService "hedge/app-services/hedge-admin/internal/service"
	"hedge/app-services/hedge-admin/models"
	comConfig "hedge/common/config"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	models2 "hedge/common/models"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/labstack/echo/v4"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	ResponseFailureMsg         = "Failed to send response content"
	RequestBodyParseFailureMsg = "Failed to parse request body"
)

type Router struct {
	edgexSdk         interfaces.ApplicationService
	ruleService      *adminService.RuleService
	appConfig        *config.AppConfig
	nodeRedService   *adminService.NodeRedService
	elasticService   *adminService.ElasticService
	vaultService     *adminService.VaultService
	nodeGroupService *adminService.NodeGroupService
	deviceService    *adminService.DeviceService
	dataMgmtService  *adminService.DataMgmtService
}

func NewRouter(service interfaces.ApplicationService, appConfig *config.AppConfig) *Router {
	router := new(Router)
	router.edgexSdk = service
	router.ruleService = adminService.NewRuleService(service, appConfig)
	router.nodeRedService = adminService.NewNodeRedService(service, appConfig)
	router.elasticService = adminService.NewElasticService(service, appConfig)
	router.vaultService = adminService.NewVaultService(service, appConfig)
	router.nodeGroupService = adminService.NewNodeGroupService(service)
	router.deviceService = adminService.NewDeviceService(service, appConfig)
	router.dataMgmtService = adminService.NewDataMgmtService(service, appConfig)
	router.appConfig = appConfig
	return router
}

func (r Router) RegisterCurrentNode() error {
	node := models2.Node{
		NodeId:   r.appConfig.NodeId,
		HostName: r.appConfig.NodeHostName,
	}
	_, err := r.nodeGroupService.SaveNode(&node)
	return err
}

func (r Router) LoadRestRoutes() {

	r.edgexSdk.LoggingClient().Info("Adding Routes")

	// Adds or Updates the node if the node exist
	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/node", interfaces.Authenticated, func(c echo.Context) error {

		return handlePostNode(c, r)
	}, http.MethodPost)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/node/:nodeName", interfaces.Authenticated, func(c echo.Context) error {

		return handleGetNode(c, r)
	}, http.MethodGet)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/node/:nodeName", interfaces.Authenticated, func(c echo.Context) error {
		return handleDeleteNode(c, r)
	}, http.MethodDelete)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/group", interfaces.Authenticated, func(c echo.Context) error {
		return handleSaveNodeGroup("", c, r)
	}, http.MethodPost)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/group/:parentGroupName", interfaces.Authenticated, func(c echo.Context) error {
		parenGroupName := c.Param("parentGroupName")
		return handleSaveNodeGroup(parenGroupName, c, r)
	}, http.MethodPost)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/group/:groupName", interfaces.Authenticated, func(c echo.Context) error {
		return handleDeleteGroup(c, r)
	}, http.MethodDelete)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/group/:parentGroupName", interfaces.Authenticated, func(c echo.Context) error {
		return handleGetGroup(c, r)
	}, http.MethodGet)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/content/import", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostImport(c, r)
	}, http.MethodPost)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/topic/:rawTopicName", interfaces.Authenticated, func(c echo.Context) error {
		return handleGetTopic(c, r)
	}, http.MethodGet)

	/* Save protocol details for a given device service in Consul of current Node and also in Redis - sync's automatically with Core
	Sample payload: {"mqtt-service":[
		{"protocolName":"prot","protocolProperties":["prop1","prop2"]},
		{"protocolName":"prot2","protocolProperties":["prop2.1","prop2.2"]}]}
	*/
	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/deviceservice/protocols", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostProtocols(c, r)
	}, http.MethodPost)

	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/deviceservice/protocols/:serviceName", interfaces.Authenticated, func(c echo.Context) error {
		return handleGetProtocols(c, r)
	}, http.MethodGet)

	// serviceName: "all" for all services or specific device service name, eg "virtual-device-service", "mqtt-service"
	_ = r.edgexSdk.AddCustomRoute("/api/v3/node_mgmt/deviceservice/initialize/:serviceName", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostInitializeService(c, r)
	}, http.MethodPost)

	// Import data (flows, rules, devices, etc) into a Node(s)
	_ = r.edgexSdk.AddCustomRoute("/api/v3/content/import/defs", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostImportDefs(c, r)
	}, http.MethodPost)

	// Export data (flows, rules, devices, etc) from Node(s)
	_ = r.edgexSdk.AddCustomRoute("/api/v3/content/export/defs", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostExportDefs(c, r)
	}, http.MethodPost)

	// Change secrets in vault
	_ = r.edgexSdk.AddCustomRoute("/api/v3/secrets/update/:secretName", interfaces.Authenticated, func(c echo.Context) error {
		return handlePostSecrets(c, r)
	}, http.MethodPost)

	// Get hedge-admin secrets from vault
	_ = r.edgexSdk.AddCustomRoute("/api/v3/secrets/:secretName", interfaces.Authenticated, func(c echo.Context) error {
		return handleGetSecrets(c, r)
	}, http.MethodGet)
}

func handlePostExportDefs(c echo.Context, r Router) *echo.HTTPError {
	var requestData dto.NodeReqData
	err := json.NewDecoder(c.Request().Body).Decode(&requestData)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, RequestBodyParseFailureMsg)
	}

	// Get the requested data from each node
	zipPath, hedgeError := r.dataMgmtService.ExportDefs(requestData, adminService.CONTENTS_FOLDER)
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	if hedgeError != nil {
		r.edgexSdk.LoggingClient().Errorf(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}
	hedgeError = r.dataMgmtService.StreamZipFile(c, zipPath)
	if hedgeError != nil {
		return hedgeError.ConvertToHTTPError()
	}

	return nil
}

func handlePostImportDefs(c echo.Context, r Router) *echo.HTTPError {
	// Get zip file from request
	file, err := r.validateImportBodySize(c)

	if err != nil {
		return err.ConvertToHTTPError()
	}

	hedgeError := r.dataMgmtService.ImportDefs(file)
	if hedgeError != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to import data: %v", hedgeError)
		return hedgeError.ConvertToHTTPError()
	}

	c.NoContent(http.StatusOK)
	return nil
}

// 1. Get all device services from Consul
// 2. For every device service, fetch protocol info from Hedge dir in consul KV - /edgex/devices/2.0/Hedge/Protocols
// 3. Create redis entry for each device
// 4. Redis device sync will get triggered automatically
func handlePostInitializeService(c echo.Context, r Router) *echo.HTTPError {
	dvServiceName := c.Param("serviceName")

	dvcServices, err := r.deviceService.InitializeDeviceServices(dvServiceName)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to initialize device services: %v", err.Error())
		return err.ConvertToHTTPError()
	}

	e := c.JSON(http.StatusOK, dvcServices)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handleGetProtocols(c echo.Context, r Router) *echo.HTTPError {
	dvServiceName := c.Param("serviceName")

	devSvcProtocols := models2.DeviceServiceProtocols{}
	devProtocols, err := r.deviceService.GetServiceProtocolsFromConsul(dvServiceName)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to get service protocols: %v", err.Error())
		return err.ConvertToHTTPError()
	}
	devSvcProtocols[dvServiceName] = devProtocols

	e := c.JSON(http.StatusOK, devSvcProtocols)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handlePostProtocols(c echo.Context, r Router) *echo.HTTPError {
	var dvSvcProtocols models2.DeviceServiceProtocols
	if err := json.NewDecoder(c.Request().Body).Decode(&dvSvcProtocols); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, RequestBodyParseFailureMsg)
	}

	err := r.deviceService.RegisterServices(dvSvcProtocols)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to register services: %v", err.Error())
		return err.ConvertToHTTPError()
	}

	//initialize the service - redis sync
	dvcServices := models2.DeviceServiceProtocols{}
	for serviceName := range dvSvcProtocols {
		dvcServicesTmp, err := r.deviceService.InitializeDeviceServices(serviceName)
		if err != nil {
			r.edgexSdk.LoggingClient().Errorf("Failed to initialize device services: %v", err.Error())
			return err.ConvertToHTTPError()
		}
		dvcServices[serviceName] = dvcServicesTmp[serviceName]
	}

	e := c.JSON(http.StatusCreated, dvcServices)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handleGetTopic(c echo.Context, r Router) *echo.HTTPError {
	rawTopicName := c.Param("rawTopicName")
	nodeTopic, err := comConfig.BuildTargetNodeTopicName(r.appConfig.NodeId, rawTopicName)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	e := c.JSON(http.StatusOK, nodeTopic)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handlePostImport(c echo.Context, r Router) *echo.HTTPError {
	var content models.ContentData
	err := json.NewDecoder(c.Request().Body).Decode(&content)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, RequestBodyParseFailureMsg)
	}

	go func() {
		//NOTE: make all imports for mandatory services below this
		err = r.ruleService.ImportStreamsAndRules(content)
		if err != nil {
			r.edgexSdk.LoggingClient().Warn(err.Error())
		}

		// Setup mqtt credentials for node-red
		r.edgexSdk.LoggingClient().Infof("Configure the mqtt node in node-red...")
		err = r.nodeRedService.ImportNodeRedWorkFlows(content, false) //installOpt=false means don't install optional demo flows
		if err != nil {
			r.edgexSdk.LoggingClient().Warn(err.Error())
		}

		//Check if env variable COMPOSE_PROFILES is set
		prof, found := os.LookupEnv("COMPOSE_PROFILES")
		if !found || len(prof) == 0 {
			r.edgexSdk.LoggingClient().Infof("Environment variable COMPOSE_PROFILES not set. Skipping further imports for optional services..")
			return
		}

		//NOTE: make all imports for optional services below this

		//if profile = all or demo, then import the node-red demo content
		if strings.Contains(prof, "all") || strings.Contains(prof, "demo") {
			err = r.nodeRedService.ImportNodeRedWorkFlows(content, true)
			if err != nil {
				r.edgexSdk.LoggingClient().Warn(err.Error())
			}
			r.edgexSdk.LoggingClient().Infof("Imported the node-red demo content...")
		}

		//if profile = all or demo or es, then import the ES demo content
		if strings.Contains(prof, "all") || strings.Contains(prof, "demo") || strings.Contains(prof, "es") {
			err = r.elasticService.SeedElasticData(content)
			if err != nil {
				r.edgexSdk.LoggingClient().Warn(err.Error())
			}
		} else {
			r.edgexSdk.LoggingClient().Infof("Skipped importing the ES demo content...")
		}
	}()

	e := c.String(http.StatusCreated, "Content imported successfully")
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handleGetGroup(c echo.Context, r Router) *echo.HTTPError {
	parenGroupName := c.Param("parentGroupName")

	if parenGroupName == "all" {
		parenGroupName = ""
	}

	_ = c.QueryParam("groupwalk")

	nodeGroups, err := r.nodeGroupService.GetNodeGroups(parenGroupName)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to get node groups: %v", err.Error())
		return err.ConvertToHTTPError()
	} else {
		e := c.JSON(http.StatusOK, nodeGroups)
		if e != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
		}
	}

	return nil
}

func handleDeleteGroup(c echo.Context, r Router) error {
	groupName := c.Param("groupName")
	err := r.nodeGroupService.DeleteNodeGroup(groupName)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to delete node group: %v", err.Error())
		return err.ConvertToHTTPError()
	}

	e := c.NoContent(http.StatusNoContent)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handleDeleteNode(c echo.Context, r Router) *echo.HTTPError {
	nodeName := c.Param("nodeName")

	err := r.nodeGroupService.DeleteNode(nodeName)

	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to delete node %s: %v", nodeName, err.Error())
		return err.ConvertToHTTPError()
	} else {
		e := c.NoContent(http.StatusNoContent)
		if e != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
		}
	}

	return nil
}

func handleGetNode(c echo.Context, r Router) *echo.HTTPError {
	nodeName := c.Param("nodeName")

	var nodes interface{}
	var err hedgeErrors.HedgeError

	if nodeName == "all" || nodeName == "All" {
		nodes, err = r.nodeGroupService.GetNodes()
	} else {
		if nodeName == "current" {
			nodeName = r.appConfig.NodeId
		}
		nodes, err = r.nodeGroupService.GetNode(nodeName)
	}

	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to get node %s: %v", nodeName, err.Error())
		return err.ConvertToHTTPError()
	}

	e := c.JSON(http.StatusOK, nodes)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}

func handlePostNode(c echo.Context, r Router) *echo.HTTPError {
	var node models2.Node
	if err := json.NewDecoder(c.Request().Body).Decode(&node); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, RequestBodyParseFailureMsg)
	}

	addToDefaultGroup := c.QueryParam("addToDefaultGroup")

	key, err := r.nodeGroupService.SaveNode(&node)
	if err != nil {
		return err.ConvertToHTTPError()
	} else {
		if addToDefaultGroup == "true" {
			err = r.nodeGroupService.AddNodeToDefaultGroup(r.appConfig.DefaultNodeGroupName, r.appConfig.DefaultNodeGroupDisplayName, &node)
			if err != nil {
				r.edgexSdk.LoggingClient().Errorf("Failed to add node to default group: %v", err.Error())
				return err.ConvertToHTTPError()
			}
		}

		err := c.String(http.StatusCreated, key)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
		}
	}

	return nil
}

func (r Router) validateImportBodySize(c echo.Context) (*multipart.FileHeader, hedgeErrors.HedgeError) {
	r.edgexSdk.LoggingClient().Infof("Allowed max file size is %dMB", r.appConfig.MaxImportExportFileSizeMB)
	MaxImportExportFileSizeBytes := int64(r.appConfig.MaxImportExportFileSizeMB) * 1024 * 1024
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, MaxImportExportFileSizeBytes)
	file, err := c.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "http: request body too large" {
			r.edgexSdk.LoggingClient().Errorf("File size exceeds limit of %dMB: %v", r.appConfig.MaxImportExportFileSizeMB, err)
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeRequestEntityTooLarge, "File size exceeds limit of "+strconv.Itoa(r.appConfig.MaxImportExportFileSizeMB)+"MB")
		}

		r.edgexSdk.LoggingClient().Errorf("Error getting file from POST: %v", err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, http.StatusText(http.StatusBadRequest))
	}
	return file, nil
}

func handleSaveNodeGroup(parenGroupName string, c echo.Context, r Router) *echo.HTTPError {
	var nodeGroup models2.NodeGroup
	if err := json.NewDecoder(c.Request().Body).Decode(&nodeGroup); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, RequestBodyParseFailureMsg)
	}

	key, err := r.nodeGroupService.SaveNodeGroup(parenGroupName, &nodeGroup)
	if err != nil {
		return err.ConvertToHTTPError()
	} else {
		err := c.String(http.StatusCreated, key)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
		}
	}

	return nil
}

func handlePostSecrets(c echo.Context, r Router) *echo.HTTPError {
	var secretData []models.VaultSecretData
	secretName := c.Param("secretName")
	if err := json.NewDecoder(c.Request().Body).Decode(&secretData); err != nil {
		r.edgexSdk.LoggingClient().Errorf(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	err := r.vaultService.UpdateSecretsInVault(secretName, secretData)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf(err.Error())
		return err.ConvertToHTTPError()
	}
	return nil
}

func handleGetSecrets(c echo.Context, r Router) *echo.HTTPError {
	secretName := c.Param("secretName") //all will fetch all hedge-admin secrets

	secrets, err := r.vaultService.GetSecretsFromVault(secretName)
	if err != nil {
		r.edgexSdk.LoggingClient().Errorf("Failed to get Vault secrets: %s", err.Error())
		return err.ConvertToHTTPError()
	}

	e := c.JSON(http.StatusOK, secrets)
	if e != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, ResponseFailureMsg)
	}

	return nil
}
