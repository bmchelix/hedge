/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"bytes"
	"encoding/json"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/labstack/echo/v4"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	servc "hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/app-services/metadata-notifier/pkg"
	"hedge/common/client"
	"hedge/common/config"
	"hedge/common/db"
	"hedge/common/dto"
	"io"
	"net/http"
	"os"
)

type DtRouter struct {
	service   interfaces.ApplicationService
	dtservice servc.DigitalTwinService
}

func NewDtRouter(srv interfaces.ApplicationService) *DtRouter {
	router := new(DtRouter)
	router.service = srv
	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(srv)
	dbClient := redis.NewDBClient(dbConfig, srv.LoggingClient())
	metadataServiceUrl := getMetadataServiceUrl(srv)
	hedgeAdminUrl := getHedgeAdminServiceUrl(srv)
	currentNodeId, _ := config.GetCurrentNodeIdAndHost(srv)
	metaservice := servc.NewMetaService(srv, dbConfig, dbClient, metadataServiceUrl, currentNodeId, hedgeAdminUrl)
	router.dtservice = servc.NewDtService(srv, dbClient, metaservice)
	err := router.dtservice.CreateIndex()
	if err != nil {
		srv.LoggingClient().Errorf("Error creating db index: %v", err)
		srv.LoggingClient().Info("Visualization API will NOT WORK with current setup!!!")
		//return nil
	}
	return router
}

func (r DtRouter) LoadDtRoutes() {
	// Scene endpoints
	r.service.AddCustomRoute("/api/v3/devices/dtwin/scene/:sceneId", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetSceneById:: Start")
		sceneId := c.Param("sceneId")

		// Try to get scene
		dt, err := r.dtservice.GetSceneById(sceneId)
		//on error
		if err != nil {
			r.service.LoggingClient().Errorf("Error querying digital-twin from db: %v", err)
			return err.ConvertToHTTPError()
		}
		//on success
		c.JSON(http.StatusOK, dt)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetSceneById:: End")
		return nil
	}, http.MethodGet)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/scene", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: Start")
		var scene dto.Scene

		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}
		err = json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&scene)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}
		// Check all fields have values
		hErr := r.dtservice.CheckScene(scene)
		if hErr != nil {
			r.service.LoggingClient().Error(hErr.Error())
			return hErr.ConvertToHTTPError()
		}
		// Try to create the scene
		hErr = r.dtservice.CreateDigitalTwin(scene)
		// on error
		if hErr != nil {
			r.service.LoggingClient().Errorf("Error creating digital-twin: %s", hErr.Error())
			return hErr.ConvertToHTTPError()
		}
		//on success
		r.notifySync(pkg.SceneType, pkg.AddAction)
		c.JSON(http.StatusOK, scene)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: End")
		return nil
	}, http.MethodPost)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/scene", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: Start")
		var scene dto.Scene

		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}
		err = json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&scene)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), err)
		}
		// Check all fields have values
		hErr := r.dtservice.CheckScene(scene)
		if hErr != nil {
			r.service.LoggingClient().Error(err.Error())
			return hErr.ConvertToHTTPError()
		}
		// Try updating the scene
		hErr = r.dtservice.UpdateDigitalTwin(scene)
		// on error
		if hErr != nil {
			r.service.LoggingClient().Errorf("Error updating digital-twin in db: %v", err)
			return hErr.ConvertToHTTPError()
		}
		//on success
		r.notifySync(pkg.SceneType, pkg.UpdateAction)
		c.JSON(http.StatusOK, scene)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: End")
		return nil
	}, http.MethodPut)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/scene/:sceneId", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: Start")
		sceneId := c.Param("sceneId")

		// Try to delete scene
		hErr := r.dtservice.DeleteDigitalTwin(sceneId)
		// on error
		if hErr != nil {
			r.service.LoggingClient().Errorf("Error deleting digital-twin from db: %v", hErr.Error())
			return hErr.ConvertToHTTPError()
		}
		//on success
		r.notifySync(pkg.SceneType, pkg.DeleteAction)
		c.JSON(http.StatusOK, sceneId)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UpsertDevice:: End")
		return nil
	}, http.MethodDelete)

	// Device endpoints
	r.service.AddCustomRoute("/api/v3/devices/dtwin/device/:deviceId", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetDeviceById:: Start")

		deviceId := c.Param("deviceId")
		metrics := c.QueryParam("metrics")
		dt, err := r.dtservice.GetDeviceById(deviceId, metrics)
		//if error
		if err != nil {
			r.service.LoggingClient().Errorf("Error querying digital-twin from db: %v", err)
			return err.ConvertToHTTPError()
		}

		//on success
		c.JSON(http.StatusOK, dt)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetDeviceById:: End")
		return nil
	}, http.MethodGet)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/device/:deviceId/scene", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetDeviceById:: Start")

		deviceId := c.Param("deviceId")
		dt, err := r.dtservice.GetDTwinByDevice(deviceId)
		// on error
		if err != nil {
			r.service.LoggingClient().Errorf("Error querying digital-twin from db: %v", err)
			return err.ConvertToHTTPError()
		}
		//on success
		c.JSON(http.StatusOK, dt)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetDeviceById:: End")
		return nil
	}, http.MethodGet)

	// Image endpoints
	r.service.AddCustomRoute("/api/v3/devices/dtwin/image", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetImageById:: Start")

		imageId := c.QueryParam("imageId")
		deviceId := c.QueryParam("deviceId")
		snapshot := c.QueryParam("snapshot")
		profileId := c.QueryParam("profileId")
		imageObj := dto.Image{
			ImgId:   imageId,
			Object:  deviceId,
			ObjName: profileId,
		}
		if imageId == "" && deviceId == "" && profileId == "" {
			r.service.LoggingClient().Error("No value was passed")
			return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}
		path, mimeType, err := r.dtservice.GetImage(imageObj)
		if imageId == "" {
			imageId = r.dtservice.GetImageId(path)
		}

		//if error
		if err != nil {
			r.service.LoggingClient().Errorf("Error querying digital-twin from db: %s", err.Error())
			return err.ConvertToHTTPError()
		}
		if snapshot == "true" {
			path, mimeType, err = r.dtservice.SnapshotImage(path, imageId, mimeType)
			if err != nil {
				r.service.LoggingClient().Errorf("Error creating snapshot: %s", err.Error())
				return err.ConvertToHTTPError()
			}
			defer os.RemoveAll(path)
		}
		f, er := os.Open(path)
		if er != nil {
			r.service.LoggingClient().Errorf("Error opening image: %v", er)
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}

		//on success
		c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename="+imageId)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() GetImageById:: End")
		return c.Stream(http.StatusOK, mimeType, f)
	}, http.MethodGet)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/image", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UploadImage:: Start")

		r.service.LoggingClient().Infof("Size is %d", c.Request().ContentLength)
		// Get data from POST
		name := c.FormValue("name")
		object := c.FormValue("object")
		imageId := c.FormValue("imageId")
		image, err := c.FormFile("file")
		if err != nil {
			r.service.LoggingClient().Errorf("Error getting file from POST: %s", err.Error())
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		imageObj := dto.Image{
			ImgId:   imageId,
			Object:  object,
			ObjName: name,
		}

		// Save image
		hErr := r.dtservice.UploadImage(image, imageObj)
		if hErr != nil {
			r.service.LoggingClient().Errorf("Error saving image: %s", hErr.Error())
			return echo.NewHTTPError(http.StatusBadRequest, hErr.Error())
		}

		//on success
		r.notifySync(pkg.ImageType, pkg.AddAction)
		c.JSON(http.StatusOK, imageId)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() UploadImage:: End")
		return nil
	}, http.MethodPost)

	r.service.AddCustomRoute("/api/v3/devices/dtwin/image/:imageId", interfaces.Authenticated, func(c echo.Context) error {
		r.service.LoggingClient().Debugf("router.AddCustomRoute() DeleteImage:: Start")

		// Get data from POST
		imageId := c.Param("imageId")
		// Delete image
		err := r.dtservice.DeleteImage(imageId)
		if err != nil {
			r.service.LoggingClient().Errorf("Error deleting image: %s", err.Error())
			return err.ConvertToHTTPError()
		}

		//on success
		r.notifySync(pkg.SceneType, pkg.DeleteAction)
		c.JSON(http.StatusOK, imageId)
		r.service.LoggingClient().Debugf("router.AddCustomRoute() DeleteImage:: End")
		return nil
	}, http.MethodDelete)
}

func (r DtRouter) notifySync(eventType, action string) error {
	topic, err := r.service.GetAppSetting("SystemPublishTopic")
	if err != nil {
		r.service.LoggingClient().Errorf("failed to get app setting err: %s", err.Error())
		return err
	}

	e := dtos.SystemEvent{
		Owner:  client.HedgeDeviceExtnsServiceKey,
		Action: action,
		Type:   eventType,
	}

	err = r.service.PublishWithTopic(topic, e, common.ContentTypeJSON)
	if err != nil {
		r.service.LoggingClient().Errorf("failed sending state sync system event err: %s", err.Error())
		return err
	}
	return nil
}
