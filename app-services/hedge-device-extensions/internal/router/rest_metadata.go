/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"hedge/app-services/hedge-device-extensions/internal/util"
	"hedge/app-services/hedge-device-extensions/pkg/service"
	dto2 "hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	models2 "hedge/common/models"
	comService "hedge/common/service"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dlclark/regexp2"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	echo "github.com/labstack/echo/v4"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"strings"
	"time"
)

type ResultId struct {
	Id string `json:"id"`
}

type ResultDeviceName struct {
	DeviceName string `json:"deviceName"`
}

type ResultDeviceSummary struct {
	Data []models2.DeviceSummary `json:"data"`
	Page models2.Page            `json:"page"`
}

const (
	MaxDeviceResourceNameLength = 35
	MaxDeviceNameLength         = 25
)

func restCreateCompleteProfile(c echo.Context, r Router) *echo.HTTPError {
	profileName := c.Param(profileName)
	r.service.LoggingClient().Infof("Create wrapper profile %s", profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var profileObject dto2.ProfileObject
	err = decoder.Decode(&profileObject)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	for _, resource := range profileObject.Profile.DeviceResources {
		if len(resource.Name) > MaxDeviceResourceNameLength {
			errMsg := fmt.Sprintf("Device resource name should not exceed %v characters in length, failed on name: %s", MaxDeviceResourceNameLength, resource.Name)
			r.service.LoggingClient().Error(errMsg)
			return echo.NewHTTPError(http.StatusBadRequest, errMsg)
		}
	}

	profId, hedgeError := r.metaService.CreateCompleteProfile(profileName, profileObject)
	if hedgeError != nil {
		r.service.LoggingClient().Error(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}

	ret := ResultId{Id: profId}
	err = c.JSON(http.StatusOK, ret)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restUpdateCompleteProfile(c echo.Context, r Router) *echo.HTTPError {
	profileName := c.Param(profileName)
	r.service.LoggingClient().Infof("Update wrapper profile %s", profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var profileObject dto2.ProfileObject
	err = decoder.Decode(&profileObject)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	for _, resource := range profileObject.Profile.DeviceResources {
		if len(resource.Name) > MaxDeviceResourceNameLength {
			errMsg := fmt.Sprintf("Device resource name should not exceed %v characters in length, failed on name: %s", MaxDeviceResourceNameLength, resource.Name)
			r.service.LoggingClient().Error(errMsg)
			return echo.NewHTTPError(http.StatusBadRequest, errMsg)
		}
	}

	mappedDevices := r.deviceInfoService.GetDevicesByProfile(profileName)
	if len(mappedDevices) > 0 {
		oldProfWrapper, err := r.metaService.GetCompleteProfile(profileName)
		//check metrics for renaming or deletion
		if err != nil {
			r.service.LoggingClient().Error("Error occurred when updating profile %s: %s", profileName, err.Error())
			return err.ConvertToHTTPError()
		}

		if profileObject.Profile.Id != "" {

			newMetrics := profileObject.Profile.DeviceResources
			oldMetrics := oldProfWrapper.Profile.DeviceResources

			newMetMap := make(map[string]string)
			for _, v := range newMetrics {
				newMetMap[v.Name] = ""
			}

			del := false
			var missingMetrics []string
			for _, v := range oldMetrics {
				if _, ok := newMetMap[v.Name]; !ok {
					missingMetrics = append(missingMetrics, v.Name)
					del = true
				}
			}
			if del {
				errMsg := fmt.Sprintf("Not allowed to delete or rename metric(s) (%v) because profile %v has associated devices", strings.Join(missingMetrics, ", "), profileName)
				r.service.LoggingClient().Error(errMsg)
				return echo.NewHTTPError(http.StatusBadRequest, errMsg)
			}

			//check commands for renaming or deletion
			newCommands := profileObject.Profile.DeviceCommands
			oldCommands := oldProfWrapper.Profile.DeviceCommands

			newComMap := make(map[string]string)
			for _, v := range newCommands {
				newComMap[v.Name] = ""
			}

			del = false
			var missingCom []string
			for _, v := range oldCommands {
				if _, ok := newComMap[v.Name]; !ok {
					missingCom = append(missingCom, v.Name)
					del = true
				}
			}
			if del {
				errMsg := fmt.Sprintf("Not allowed to delete or rename command(s) (%v) because profile %v has associated devices", strings.Join(missingCom, ", "), profileName)
				r.service.LoggingClient().Error(errMsg)
				return echo.NewHTTPError(http.StatusBadRequest, errMsg)
			}
		}

		if profileObject.DeviceAttributes != nil {

			//check device attributes for renaming or deletion
			newExtensions := profileObject.DeviceAttributes
			oldExtensions := oldProfWrapper.DeviceAttributes

			newExtMap := make(map[string]string)
			for _, v := range newExtensions {
				newExtMap[v.Field] = ""
			}

			del := false
			var missingExt []string
			for _, v := range oldExtensions {
				if _, ok := newExtMap[v.Field]; !ok {
					missingExt = append(missingExt, v.Field)
					del = true
				}
			}

			if del {
				//get all devices from current profile
				for _, deviceName := range mappedDevices {
					err := r.metaService.DeleteDeviceExtension(deviceName, missingExt)
					if err != nil {
						r.service.LoggingClient().Error(err.Error())
						return err.ConvertToHTTPError()
					}
				}
			}
		}

		if profileObject.Profile.Id == "" && profileObject.DeviceAttributes == nil {
			errMsg := fmt.Sprintf("Request must contain profile field and/or extensions field")
			r.service.LoggingClient().Error(errMsg)
			return echo.NewHTTPError(http.StatusBadRequest, errMsg)
		}
	}

	profId, hedgeError := r.metaService.UpdateCompleteProfile(profileName, profileObject)
	if hedgeError != nil {
		r.service.LoggingClient().Errorf("Error occurred when updating profile %s: %v", profileName, hedgeError)
		return hedgeError.ConvertToHTTPError()
	}

	ret := ResultId{Id: profId}

	errC := c.JSON(http.StatusOK, ret)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restGetCompleteProfile(c echo.Context, r Router) *echo.HTTPError {
	profileName := c.Param(profileName)
	r.service.LoggingClient().Debugf("Get wrapper profile %s", profileName)

	var fullProfile dto2.ProfileObject
	fullProfile, err := r.metaService.GetCompleteProfile(profileName)
	if err != nil {
		r.service.LoggingClient().Error(fmt.Sprintf("Error occurred when retrieving profile %s: %s", profileName, err.Error()))
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, fullProfile)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restDeleteCompleteProfile(c echo.Context, r Router) *echo.HTTPError {
	profileName := c.Param(profileName)
	r.service.LoggingClient().Infof("Delete wrapper profile %s", profileName)

	err := r.metaService.DeleteCompleteProfile(profileName)
	if err != nil {
		r.service.LoggingClient().Errorf("Error occurred when deleting profile %s: %s", profileName, err.Error())
		return err.ConvertToHTTPError()
	}

	if err := c.NoContent(http.StatusOK); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	_ = c.NoContent(http.StatusOK)
	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restGetProfileTags(c echo.Context, r Router) *echo.HTTPError {
	names, err := util.GetProfileNames(r.metaDataServiceURL, r.service.LoggingClient())
	if err != nil {
		r.service.LoggingClient().Error(fmt.Sprintf("Error occurred when retrieving profile names: %s", err.Error()))
		return err.ConvertToHTTPError()
	}
	var tags []string
	for _, name := range names {
		fullProfile, err := r.metaService.GetCompleteProfile(name)
		if err != nil {
			r.service.LoggingClient().Error(fmt.Sprintf("Error occurred when retrieving full profile %s: %s", name, err.Error()))
			return err.ConvertToHTTPError()
		}
		for _, deviceAttr := range fullProfile.DeviceAttributes {
			if !slices.Contains(tags, deviceAttr.Field) {
				tags = append(tags, deviceAttr.Field)
			}
		}
		for _, contextualAttr := range fullProfile.ContextualAttributes {
			if !slices.Contains(tags, contextualAttr) {
				tags = append(tags, contextualAttr)
			}
		}
	}

	errC := c.JSON(http.StatusOK, tags)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restGetProfilesByTags(c echo.Context, r Router) *echo.HTTPError {
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	tags := []string{}
	err = decoder.Decode(&tags)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	names, hedgeErr := util.GetProfileNames(r.metaDataServiceURL, r.service.LoggingClient())
	if hedgeErr != nil {
		r.service.LoggingClient().Error(fmt.Sprintf("Error occurred when retrieving profile names: %s", hedgeErr.Error()))
		return hedgeErr.ConvertToHTTPError()
	}
	var profiles []dto2.ProfileObject
	for _, name := range names {
		fullProfile, hedgeErr := r.metaService.GetCompleteProfile(name)
		if hedgeErr != nil {
			r.service.LoggingClient().Error(fmt.Sprintf("Error occurred when retrieving full profile %s: %s", name, hedgeErr.Error()))
			return hedgeErr.ConvertToHTTPError()
		}
		var found bool
		for _, deviceAttr := range fullProfile.DeviceAttributes {
			if slices.Contains(tags, deviceAttr.Field) {
				profiles = append(profiles, fullProfile)
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, contextualAttr := range fullProfile.ContextualAttributes {
			if slices.Contains(tags, contextualAttr) {
				profiles = append(profiles, fullProfile)
				found = true
				break
			}
		}
		if found {
			continue
		}
	}
	errC := c.JSON(http.StatusOK, profiles)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
	return nil
}

func restCreateCompleteDevice(c echo.Context, r Router) *echo.HTTPError {
	deviceName := c.Param(deviceName)
	r.service.LoggingClient().Infof("Create wrapper device %s", deviceName)

	errorMessage := fmt.Sprintf("Error creating complete device %s", deviceName)

	pattern := regexp2.MustCompile("[^a-z0-9_~-]", regexp2.IgnoreCase)
	notValid, err := pattern.MatchString(deviceName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	if notValid || len(deviceName) > MaxDeviceNameLength {
		errMsg := fmt.Sprintf("Name field only allows alphanumeric characters and special characters -_~ and should not exceed %v characters in length", MaxDeviceNameLength)
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, errMsg)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%s: %v", errorMessage, errMsg))
	}

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var deviceObject dto2.DeviceObject
	err = decoder.Decode(&deviceObject)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
	}

	devcId, hedgeError := r.metaService.CreateCompleteDevice(deviceName, deviceObject)
	if hedgeError != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return hedgeError.ConvertToHTTPError()
	}

	ret := ResultId{Id: devcId}
	err = c.JSON(http.StatusOK, ret)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restUpdateCompleteDevice(c echo.Context, r Router) *echo.HTTPError {
	deviceName := c.Param(deviceName)
	r.service.LoggingClient().Infof("Update wrapper device %s", deviceName)

	errorMessage := fmt.Sprintf("Error updating complete device %s", deviceName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var deviceObject dto2.DeviceObject
	err = decoder.Decode(&deviceObject)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
	}

	devcId, hedgeError := r.metaService.UpdateCompleteDevice(deviceName, deviceObject)
	if hedgeError != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, hedgeError)
		return hedgeError.ConvertToHTTPError()
	}

	ret := ResultId{Id: devcId}
	err = c.JSON(http.StatusOK, ret)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restGetCompleteDevice(c echo.Context, r Router) *echo.HTTPError {
	deviceName := c.Param(deviceName)
	metrics := c.QueryParam("metrics")
	r.service.LoggingClient().Debugf("Get wrapper device %s", deviceName)

	errorMessage := fmt.Sprintf("Error getting complete device %s", deviceName)

	var fullDevice dto2.DeviceObject
	fullDevice, err := r.metaService.GetCompleteDevice(deviceName, metrics, r.service)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, fullDevice)
	if errC != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

// TODO: edgeNode is never used actually
func restDeleteCompleteDevice(c echo.Context, r Router) *echo.HTTPError {
	deviceName := c.Param(deviceName)
	r.service.LoggingClient().Infof("Delete wrapper device %s", deviceName)

	errorMessage := fmt.Sprintf("Error deleting complete device %s", deviceName)

	err := r.metaService.DeleteCompleteDevice(deviceName)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err.ConvertToHTTPError()
	}

	errC := c.NoContent(http.StatusOK)
	if errC != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	//if success, reload cache asynchronously
	go r.deviceInfoService.LoadDeviceInfoFromDB()
	return nil
}

func restAddAssociations(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Save Associations for a given node and type")

	errorMessage := fmt.Sprintf("Error adding associations for a given node and type: %v", c.Param("type"))

	nodeAType := c.Param("nodeAType")
	if nodeAType != "device" {
		errMsg := fmt.Sprintf("Node of type %s not supported", nodeAType)
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, errMsg)
		return echo.NewHTTPError(http.StatusNotImplemented, fmt.Sprintf("%s: %v", errorMessage, errMsg))
	}

	nodeAName := c.Param("nodeAName")
	r.service.LoggingClient().Debugf("Adding association for %s of type %s", nodeAName, nodeAType)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, errorMessage)
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var associatedNodes []dto2.AssociationNode
	err = decoder.Decode(&associatedNodes)
	if len(associatedNodes) > 0 {
		id, err := r.metaService.AddAssociation(nodeAName, nodeAType, associatedNodes)
		if err != nil {
			r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, id)
		if errC != nil {
			r.service.LoggingClient().Errorf("%s: %v", errorMessage, errC)
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	} else {
		r.service.LoggingClient().Infof("Empty associations passed. Removing all..")
		hedgeError := r.metaService.DeleteAssociation(nodeAName, nodeAType)

		if hedgeError != nil {
			if hedgeError.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				r.service.LoggingClient().Infof("Device association not found. Continuing ahead..")
			} else {
				r.service.LoggingClient().Errorf("%s: %v", errorMessage, hedgeError)
				return hedgeError.ConvertToHTTPError()
			}

		}

		err := c.JSON(http.StatusOK, []string{})
		if err != nil {
			r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}

	return nil
}

func restGetAssociations(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Associations for a given node and type")

	nodeAType := c.Param("nodeAType")

	errorMessage := fmt.Sprintf("Error getting associations for a given node and type: %v", c.Param("type"))

	if nodeAType != "device" {
		errMsg := fmt.Sprintf("Node of type %s not suported", nodeAType)
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, errMsg)
		return echo.NewHTTPError(http.StatusNotImplemented, fmt.Sprintf("%s: %v", errorMessage, errMsg))
	}

	nodeAName := c.Param("nodeAName")
	r.service.LoggingClient().Debugf("Getting associations for %s", nodeAName)
	association, err := r.metaService.GetAssociation(nodeAName)

	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, association)
	if errC != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restUpdateAssociations(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Update Associations for a given node and type")

	nodeAType := c.Param("nodeAType")
	nodeAName := c.Param("nodeAName")

	errorMessage := fmt.Sprintf("Error updating associations for a given node type: %s and node name %s", nodeAType, nodeAName)

	if nodeAType != "device" {
		errMsg := fmt.Sprintf("Node of type %s not supported", nodeAType)
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, errMsg)
		return echo.NewHTTPError(http.StatusNotImplemented, fmt.Sprintf("%s: %v", errorMessage, errMsg))
	}

	r.service.LoggingClient().Debugf("Updating association for %s", nodeAName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, err)
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var associatedNodes []dto2.AssociationNode

	err = decoder.Decode(&associatedNodes)
	if len(associatedNodes) > 0 {
		id, err := r.metaService.UpdateAssociation(nodeAName, nodeAType, associatedNodes, false)
		if err != nil {
			r.service.LoggingClient().Errorf(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, ResultId{Id: id})
		if errC != nil {
			r.service.LoggingClient().Errorf(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	} else {
		r.service.LoggingClient().Infof("Empty associations passed. Removing all..")
		err := r.metaService.DeleteAssociation(nodeAName, nodeAType)

		if err != nil {
			if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
				r.service.LoggingClient().Infof("Device association not found. Continuing..")
			} else {
				return err.ConvertToHTTPError()
			}
		}

		errC := c.JSON(http.StatusOK, []string{})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}

	return nil
}

func restDeleteAssociations(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Delete Associations for a given node and type")

	nodeAType := c.Param("nodeAType")
	nodeAName := c.Param("nodeAName")

	errorMessage := fmt.Sprintf("Error deleting associations for a given node type %s and node name %s", nodeAType, nodeAName)

	if nodeAType != "device" {
		errMsg := fmt.Sprintf("Node of type %s not supported", nodeAType)
		r.service.LoggingClient().Errorf("%s: %v", errorMessage, errMsg)
		return echo.NewHTTPError(http.StatusNotImplemented, fmt.Sprintf("%s: %v", errorMessage, errMsg))
	}

	r.service.LoggingClient().Debugf("Adding association for %s", nodeAName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var associatedNodes []dto2.AssociationNode

	err = decoder.Decode(&associatedNodes)
	// err intentionally left unprocessed
	//if err != nil {
	//	r.service.LoggingClient().Error(err.Error())
	//	return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	//}

	hedgeError := r.metaService.DeleteAssociation(nodeAName, nodeAType)

	if hedgeError != nil {
		if hedgeError.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().Infof("Device association not found. Continuing ahead..")
		} else {
			r.service.LoggingClient().Error(hedgeError.Error())
			return hedgeError.ConvertToHTTPError()
		}
	}

	err = c.NoContent(http.StatusOK)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restAddDvcProfileExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Create device profile extension")

	profileName := c.Param(profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))

	var deviceExts []dto2.DeviceExtension

	err = decoder.Decode(&deviceExts)
	if len(deviceExts) > 0 {
		id, err := r.metaService.AddDeviceExtensionsInProfile(profileName, deviceExts)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, ResultId{Id: id})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}
	return nil
}

func restUpdateDvcProfileExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Update device extension in profile")

	profileName := c.Param(profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))

	var deviceExtNew []dto2.DeviceExtension

	err = decoder.Decode(&deviceExtNew)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	deviceExtOld, err := r.metaService.GetDeviceExtensionInProfile(profileName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	//check if profile has devices before updating attributes
	mappedDevices := r.deviceInfoService.GetDevicesByProfile(profileName)
	if len(mappedDevices) > 0 {
		devNewMap := make(map[string]string)
		for _, v := range deviceExtNew {
			devNewMap[v.Field] = ""
		}

		//check if an attribute has been deleted
		del := false
		var missingAttr []string
		for _, v := range deviceExtOld {
			if _, ok := devNewMap[v.Field]; !ok {
				missingAttr = append(missingAttr, v.Field)
				del = true
			}
		}

		if del {
			errMsg := fmt.Sprintf("Not allowed to delete or rename attribute(s) (%v) because profile %v has associated devices", strings.Join(missingAttr, ", "), profileName)
			r.service.LoggingClient().Error(errMsg)
			return echo.NewHTTPError(http.StatusBadRequest, errMsg)
		}
	}

	if len(deviceExtNew) > 0 {
		id, err := r.metaService.UpdateDeviceExtensionsInProfile(profileName, deviceExtNew, true)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, ResultId{Id: id})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	} else {
		r.service.LoggingClient().Infof("Empty device extension in profile list passed. Removing all..")
		err := r.metaService.DeleteDeviceExtensionInProfile(profileName)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, []string{})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}

	return nil
}

func restGetDvcProfileExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Device Profile extension")

	profileName := c.Param(profileName)

	devExt, err := r.metaService.GetDeviceExtensionInProfile(profileName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		devExt = make([]dto2.DeviceExtension, 0)
	} else if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, devExt)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restDeleteDvcProfileExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Delete Device Profile extension")

	profileName := c.Param(profileName)

	mappedDevices := r.deviceInfoService.GetDevicesByProfile(profileName)
	if len(mappedDevices) > 0 {
		errMsg := fmt.Sprintf("Not allowed to delete attributes because profile %v has associated devices", profileName)
		r.service.LoggingClient().Error(errMsg)
		return echo.NewHTTPError(http.StatusBadRequest, errMsg)
	}

	err := r.metaService.DeleteDeviceExtensionInProfile(profileName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	errC := c.NoContent(http.StatusOK)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restAddDeviceExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Create device extension")

	deviceName := c.Param(deviceName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))

	var deviceExts []dto2.DeviceExt
	err = decoder.Decode(&deviceExts)
	if len(deviceExts) > 0 {
		id, err := r.metaService.AddDeviceExtension(deviceName, deviceExts)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, ResultId{Id: id})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}

	return nil
}

func restUpdateDeviceExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Update device extension")

	deviceName := c.Param(deviceName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))

	var deviceExts []dto2.DeviceExt
	err = decoder.Decode(&deviceExts)
	if len(deviceExts) > 0 {
		id, err := r.metaService.UpdateDeviceExtension(deviceName, deviceExts, false)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, ResultId{Id: id})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	} else {
		r.service.LoggingClient().Infof("Empty device extension list passed. Removing all..")
		err := r.metaService.DeleteAllDeviceExtension(deviceName)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}

		errC := c.JSON(http.StatusOK, []string{})
		if errC != nil {
			r.service.LoggingClient().Error(errC.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}

	return nil
}

func restGetDeviceExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Device extension")

	deviceName := c.Param(deviceName)

	devExt, err := r.metaService.GetDeviceExtension(deviceName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		devExt = make([]dto2.DeviceExtResp, 0)
	} else if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, devExt)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func restDeleteAllDeviceExt(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Delete Device extension")

	deviceName := c.Param(deviceName)

	err := r.metaService.DeleteAllDeviceExtension(deviceName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	errC := c.NoContent(http.StatusOK)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getDevicesSummary(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Device Summary")

	// create filter query
	query := service.DeviceQuery(c)

	summary, page, err := r.metaService.GetDevices(query)
	if err != nil {
		r.service.LoggingClient().Error("get devices failed: " + err.Error())
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, ResultDeviceSummary{
		Data: summary,
		Page: page,
	})

	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getDeviceServiceProtocols(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debugf("Get Device Service Protocols")

	svcName := c.Param(serviceName)

	protocols, err := r.metaService.GetProtocolsForService(svcName)

	if err != nil {
		if err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			r.service.LoggingClient().Debugf("Protocols not found for the Device Service %s", svcName)
		} else {
			return err.ConvertToHTTPError()
		}
	}

	dsps := models2.DeviceServiceProtocols{}
	dps := models2.DeviceProtocols{}
	for protName, protProps := range protocols {
		dp := models2.DeviceProtocol{}
		dp.ProtocolName = protName
		dp.ProtocolProperties = strings.Split(strings.Trim(protProps, "[]"), ",")
		dps = append(dps, dp)
	}
	dsps[svcName] = dps

	errC := c.JSON(http.StatusOK, dsps)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getGivenDevicesSummary(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Device summary for given devices")

	var devicesList []string
	reqBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("Error reading the request payload : %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	err = json.Unmarshal(reqBody, &devicesList)
	if err != nil {
		r.service.LoggingClient().Errorf("Error unmarshalling the request payload : %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	var deviceSummary []dtos.Device
	for _, device := range devicesList {
		deviceDetails, _, err := r.metaService.GetDeviceDetails(device)

		if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
			// If device not found then remove it from the list. Device may have been deleted.
			r.service.LoggingClient().Infof("Device not found. Looks like the device was deleted. Ignoring it: %s", device)
			continue
		} else if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}
		deviceSummary = append(deviceSummary, deviceDetails)
	}

	errC := c.JSON(http.StatusOK, deviceSummary)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func upsertProfileContextualAttributes(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Create Profile Contextual Data")
	deviceName := c.Param(profileName)
	var contextualAttributes []string

	reqBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Errorf("Error reading the request payload : %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	err = json.Unmarshal(reqBody, &contextualAttributes)
	if err != nil {
		r.service.LoggingClient().Errorf("Error unmarshalling the request payload : %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	hedgeError := r.metaService.UpsertProfileContextualAttributes(deviceName, contextualAttributes)
	if hedgeError != nil {
		r.service.LoggingClient().Errorf("Error upserting profile contextual attributes : device %s: %v", deviceName, hedgeError)
		return hedgeError.ConvertToHTTPError()
	}

	err = c.NoContent(http.StatusOK)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getProfileContextualData(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Profile Contextual Data")
	deviceName := c.Param(profileName)

	data, err := r.metaService.GetProfileContextualAttributes(deviceName)
	if err != nil {
		r.service.LoggingClient().Errorf("Error getting profile contextual attributes : device %s : %v", deviceName, err)
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, data)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func deleteProfileContextualData(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Delete Profile Contextual Data")
	deviceName := c.Param(profileName)

	err := r.metaService.DeleteProfileContextualAttributes(deviceName)
	if err != nil {
		r.service.LoggingClient().Errorf("Error Deleting Profile Contextual Data: device %s : %v", deviceName, err)
		return err.ConvertToHTTPError()
	}

	errC := c.JSON(http.StatusOK, ResultDeviceName{DeviceName: deviceName})
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func upsertDeviceContextualData(c echo.Context, r Router, reqBody []byte) *echo.HTTPError {
	r.service.LoggingClient().Debugf("Create Device Contextual Data")
	deviceName := c.Param(deviceName)
	var deviceContextualData map[string]interface{}

	if err := json.Unmarshal(reqBody, &deviceContextualData); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	if err := r.metaService.UpsertDeviceContextualAttributes(deviceName, deviceContextualData); err != nil {
		r.service.LoggingClient().Errorf("device not found: deviceName: %s", deviceName)
		return err.ConvertToHTTPError()
	}

	err := c.NoContent(http.StatusOK)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getDeviceContextualData(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Device Contextual Data")
	deviceName := c.Param(deviceName)

	contextualData, err := r.metaService.GetDeviceContextualAttributes(deviceName)
	if err != nil {
		r.service.LoggingClient().Errorf("Error Getting Device Contextual Data for device %s : %v", deviceName, err)
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, contextualData); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

// Get MetaDataSummary
func getProfileMetaDataSummary(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Profile metadata ( Contextual & device) based on profile list passed as param")
	profileNames := c.QueryParam("profiles")
	profiles := strings.Split(profileNames, ",")

	profileSummaries, err := r.metaService.GetProfileMetaDataSummary(profiles)
	if err != nil {
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, profileSummaries); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

// Get MetaDataSummary
func getRelatedProfiles(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Related profiles based on given profiles")
	profileNames := c.QueryParam("profiles")
	profiles := strings.Split(profileNames, ",")

	relatedProfileNames, err := r.metaService.GetRelatedProfiles(profiles)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, relatedProfileNames); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

// getAttributesGroupedByProfiles
func getAttributesGroupedByProfiles(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Related profiles based on given profiles")
	profileNames := c.QueryParam("profiles")
	profiles := strings.Split(profileNames, ",")
	// if there is only 1 profile, the grouping mechanism is by device instance, so don't show allow merge using attributes
	var attributesByProfileColumns map[string][]string

	if len(profiles) == 1 {
		attributesByProfileColumns = make(map[string][]string)
	} else {
		var err hedgeErrors.HedgeError
		attributesByProfileColumns, err = r.metaService.GetAttributesGroupedByProfiles(profiles)
		if err != nil {
			r.service.LoggingClient().Error(err.Error())
			return err.ConvertToHTTPError()
		}
	}

	if err := c.JSON(http.StatusOK, attributesByProfileColumns); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func deleteDeviceContextualData(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Info("Delete Device Contextual Data")
	deviceName := c.Param(deviceName)

	err := r.metaService.DeleteDeviceContextualAttributes(deviceName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, ResultDeviceName{DeviceName: deviceName}); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getDeviceServices(r Router, nodeId string) ([]string, *echo.HTTPError) {
	r.service.LoggingClient().Debug("Get Device Services")
	var deviceService []string

	serviceList, err := comService.GetDeviceService(r.metaDataServiceURL).GetDeviceService()
	if err != nil {
		r.service.LoggingClient().Errorf("Error getting Device Services: %v", err)
		return nil, err.ConvertToHTTPError()
	}

	if nodeId != "" {
		// If nodeId is CORE, return service names without node prefix
		if nodeId == r.currentNodeId {
			for _, serv := range serviceList {
				serviceDetails := serv.(map[string]interface{})
				serviceName := serviceDetails["name"].(string)
				if !strings.Contains(serviceName, "__") {
					deviceService = append(deviceService, serviceName)
				}
			}
		} else {
			deviceService = filterByNodeId(serviceList, nodeId)
		}
	} else {
		// Remove node prefix from all service names
		for _, serv := range serviceList {
			serviceDetails := serv.(map[string]interface{})
			_, serviceName := comService.SplitDeviceServiceNames(serviceDetails["name"].(string))
			deviceService = append(deviceService, serviceName)
		}
	}

	// Return only unique srvice names
	return comService.UniqueString(deviceService), nil
}

func getDeviceService(c echo.Context, r Router, svcName string) (interface{}, *echo.HTTPError) {
	r.service.LoggingClient().Debug("Get Device Service")
	resp, err := comService.GetOneDeviceService(r.metaDataServiceURL, svcName).GetDeviceService()

	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return nil, err.ConvertToHTTPError()
	}

	return resp, nil
}

func getSQLMetaData(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get SQL Metadata")
	bodyBytes, err := io.ReadAll(c.Request().Body)

	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var sqlQuery map[string]string
	err = decoder.Decode(&sqlQuery)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	sqlMetaData, hedgeError := r.metaService.GetSQLMetaData(sqlQuery["sql"])
	if hedgeError != nil {
		r.service.LoggingClient().Error(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, sqlMetaData); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func filterByNodeId(arr []interface{}, node string) []string {
	var devArr []string
	for _, v := range arr {
		serv := v.(map[string]interface{})
		if strings.HasPrefix(serv["name"].(string), node) {
			_, serviceName := comService.SplitDeviceServiceNames(serv["name"].(string))
			devArr = append(devArr, serviceName)
		}
	}
	return devArr
}

func upsertDownsamplingConfig(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Add/Update Downsampling Config")
	profileName := c.Param(profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var config *dto2.DownsamplingConfig
	err = decoder.Decode(&config)

	if config.DefaultDownsamplingIntervalSecs == 0 || config.DefaultDataCollectionIntervalSecs == 0 {
		r.service.LoggingClient().Error("DefaultDownsamplingIntervalSecs/DefaultDataCollectionIntervalSecs value not set")
		return echo.NewHTTPError(http.StatusBadRequest, "DefaultDownsamplingIntervalSecs/DefaultDataCollectionIntervalSecs value not set")
	}

	hedgeError := r.metaService.UpsertDownsamplingConfig(profileName, config)
	if hedgeError != nil {
		r.service.LoggingClient().Error(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}

	if err := c.NoContent(http.StatusOK); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getDownsamplingConfig(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Downsampling Config by profile name")
	profileName := c.Param(profileName)

	config, err := r.metaService.GetDownsamplingConfig(profileName)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, config); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func upsertAggregateDefinition(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Upsert Downsampling Config Aggregate")
	profileName := c.Param(profileName)

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var config dto2.DownsamplingConfig
	err = decoder.Decode(&config)

	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	hedgeError := r.metaService.UpsertAggregateDefinition(profileName, config)
	if hedgeError != nil {
		r.service.LoggingClient().Error(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}

	if err = c.NoContent(http.StatusOK); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func upsertNodeRawDataConfigs(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Add/Update Node Raw Data Configs")

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	var nodeRawDataConfigs []dto2.NodeRawDataConfig
	err = decoder.Decode(&nodeRawDataConfigs)

	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	// Validate configs
	errC := validateNodeRawDataConfigs(&nodeRawDataConfigs)
	if errC != nil {
		r.service.LoggingClient().Error(errC.Error())
		return errC
	}

	hedgeError := r.metaService.UpsertNodeRawDataConfigs(nodeRawDataConfigs)
	if hedgeError != nil {
		r.service.LoggingClient().Error(hedgeError.Error())
		return hedgeError.ConvertToHTTPError()
	}

	if err = c.NoContent(http.StatusOK); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func getNodeRawDataConfigs(c echo.Context, r Router) *echo.HTTPError {
	r.service.LoggingClient().Debug("Get Node Raw Data Configs by node IDs")
	// Retrieve the comma-separated nodes from the query parameter
	nodeIDsParam := c.QueryParam("nodes")
	if nodeIDsParam == "" {
		err := "'nodes' parameter should not be empty"
		r.service.LoggingClient().Error(err)
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	// Split the nodeIDsParam into a slice of strings
	noSpaces := strings.ReplaceAll(nodeIDsParam, " ", "")
	nodeIDs := strings.Split(noSpaces, ",")

	configs, err := r.metaService.GetNodeRawDataConfigs(nodeIDs)
	if err != nil {
		r.service.LoggingClient().Error(err.Error())
		return err.ConvertToHTTPError()
	}

	if err := c.JSON(http.StatusOK, configs); err != nil {
		r.service.LoggingClient().Error(err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return nil
}

func validateNodeRawDataConfigs(nodeRawDataConfigs *[]dto2.NodeRawDataConfig) *echo.HTTPError {
	seenNodeIDs := make(map[string]bool) // Map to detect duplicated configs (by NodeID)

	for i := range *nodeRawDataConfigs {
		config := &(*nodeRawDataConfigs)[i]
		// If some Node value is not provided
		if config.Node.NodeID == "" || config.Node.Host == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Node value is not provided")
		}

		// Check for duplicate NodeID
		if _, ok := seenNodeIDs[config.Node.NodeID]; ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Duplicate NodeID '%s' found at config %d", config.Node.NodeID, i))
		}
		seenNodeIDs[config.Node.NodeID] = true

		// If StartTime is 0 and SendRawData is true, then set StartTime to the current time
		if (config.StartTime == 0) && config.SendRawData {
			currentTime := time.Now().Unix()
			config.StartTime = currentTime
		}

		// If EndTime is before StartTime
		if config.SendRawData && config.EndTime != 0 && config.EndTime < config.StartTime {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Config %d has EndTime before StartTime", i))
		}

		// If EndTime is 0 and SendRawData is true, set it to StartTime + 24 hours
		if config.EndTime == 0 && config.SendRawData {
			endTime := config.StartTime + 86400 // Add 24 hours in seconds
			config.EndTime = endTime
		}

		// If SendRawData is false, set EndTime it to the current time (to start sending raw data immediately)
		if !config.SendRawData {
			currentTime := time.Now().Unix()
			config.EndTime = currentTime
		}
	}

	return nil
}
