//
// Copyright (c) 2019 Intel Corporation
// Copyright (c) 2021 IOTech Ltd
// (c) Copyright 2020-2025 BMC Software, Inc.
//
// Contributors: BMC Software, Inc. - BMC Helix Edge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package driver

import (
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	model "github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/cast"
)

const (
	apiResourceRoute  = common.ApiBase + "/resource/:" + common.DeviceName
	apiDataGenRoute   = common.ApiBase + "/dataGenerator/commands"
	handlerContextKey = "RestHandler"
)

type Reading struct {
	Name   string            `json:"name"`
	Value  interface{}       `json:"value"`
	Origin int64             `json:"origin"`
	Tags   map[string]string `json:"tags,omitempty"`
}

type ReadingData struct {
	Readings []Reading `json:"readings"`
}

// @Summary      updates the seed data generation parameters so subsequent data values are based on new seed
// @Description  You might change the average, or min, max etc depending on what you want.
// @Tags         device-virtual - simulated data generator
// @Param        Body            body     []datagenerator.Command true "array of commands."
// @Success      202             "successful"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/dataGenerator/commands [put]
func (h *VirtualDriver) addDataGenerationUpdateRoute() {
	h.sdk.AddCustomRoute(apiDataGenRoute, interfaces.Authenticated, func(c echo.Context) error {
		return h.handleCommandExecution(c)
	}, "PUT")
	h.lc.Infof("Route %s added.", apiDataGenRoute)
}

// @Summary      ingest the specified machine or sensor data for the device
// @Description  Multiple metric values can be ingested at once, origin can be optionally specified in microseconds
// @Tags         device-virtual
// @Param     deviceName  path     string  true   "Device Name"  // Name of the Device for which the data is be ingested
// @Param        Body            body     ReadingData true "ReadingData"
// @Success      200             ""
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/resource [post]
func (h *VirtualDriver) addRestDataIngestionRoute() {
	if err := h.sdk.AddCustomRoute(apiResourceRoute, interfaces.Authenticated, func(c echo.Context) error {
		return h.processAsyncRequest(c)
	}, http.MethodPost); err != nil {
		h.lc.Errorf("unable to add required route: %s: %s", apiResourceRoute, err.Error())
	} else {
		h.lc.Infof("Route %s added.", apiResourceRoute)
	}
}

func (h *VirtualDriver) processAsyncRequest(c echo.Context) error {
	deviceName := c.Param(common.DeviceName)

	h.lc.Debugf("Received POST for Device=%s", deviceName)

	contentType := c.Request().Header.Get(common.ContentType)
	if contentType != common.ContentTypeJSON {
		return c.String(http.StatusBadRequest, fmt.Sprintf("wrong Content-Type: expected %s but received '%s'", common.ContentTypeJSON, contentType))
	}

	device, err := h.sdk.GetDeviceByName(deviceName)
	if err != nil {
		h.lc.Errorf("Incoming reading ignored. Device '%s' not found", deviceName)
		return c.String(http.StatusNotFound, fmt.Sprintf("Device '%s' not found", deviceName))
	}

	var rd ReadingData
	err = json.NewDecoder(c.Request().Body).Decode(&rd)
	if err != nil {
		h.lc.Errorf("failed to parse readings data, error: %v", err)
		return c.String(http.StatusBadRequest, fmt.Sprintf("failed to parse readings data, error: %v", err))
	}

	commandValues := make([]*models.CommandValue, 0)
	resources := make(map[string]bool)
	for _, r := range rd.Readings {
		if _, exist := resources[r.Name]; exist {
			h.lc.Errorf("Incoming readings ignored. Resource '%s' has duplicate entries", r.Name)
			return c.String(http.StatusBadRequest, fmt.Sprintf("Resource '%s' has duplicate entries", r.Name))
		}

		deviceResource, ok := h.sdk.DeviceResource(deviceName, r.Name)
		if !ok {
			h.lc.Errorf("Incoming reading ignored. Resource '%s' not found")
			return c.String(http.StatusNotFound, fmt.Sprintf("Resource '%s' not found", r.Name))
		}

		value, err := validateCommandValue(deviceResource, r.Value, deviceResource.Properties.ValueType)
		if err != nil {
			h.lc.Errorf("Incoming reading ignored. Unable to parse Comamand Value for Device=%s Command=%s: %s",
				deviceName, r.Name, err)
			return c.String(http.StatusBadRequest, err.Error())
		}
		var origin int64
		if r.Origin == 0 {
			origin = time.Now().UnixNano()
		} else {
			// check whether timestamp is in ms
			if r.Origin > 1e12 && r.Origin < 1e15 {
				// Convert milliseconds to nanoseconds
				origin = r.Origin * int64(time.Millisecond)
				h.lc.Debugf("Converted Nanoseconds from miliiseconds :%d", origin)
			} else if r.Origin > 1e15 && r.Origin < 1e18 {
				origin = r.Origin * int64(time.Microsecond)
				h.lc.Debugf("Timestamp is not in milliseconds, converted to ns: %d", origin)
			} else {
				origin = r.Origin
			}
		}
		cv, err := models.NewCommandValueWithOrigin(deviceResource.Name, deviceResource.Properties.ValueType, value, origin)
		//cv, err := models.NewCommandValue(deviceResource.Name, deviceResource.Properties.ValueType, value)
		// add tags to commandValue if that was passed
		if r.Tags != nil {
			cv.Tags = r.Tags
		}
		if err != nil {
			h.lc.Errorf("Incoming reading ignored. Unable to validate Command Value for Device=%s Command=%s: %s",
				deviceName, r.Name, err)
			return c.String(http.StatusBadRequest, err.Error())
		}

		resources[r.Name] = true

		commandValues = append(commandValues, cv)
	}

	asyncValues := &models.AsyncValues{
		DeviceName:    deviceName,
		SourceName:    device.Id,
		CommandValues: commandValues,
	}

	h.lc.Debugf("Incoming reading received: Device=%s Reading=%v", deviceName, commandValues)

	h.asyncCh <- asyncValues

	return nil
}

func (h *VirtualDriver) readBody(request *http.Request) ([]byte, error) {
	defer request.Body.Close()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("no request body provided")
	}

	return body, nil
}

func deviceHandler(c echo.Context) error {
	handler, ok := c.Request().Context().Value(handlerContextKey).(VirtualDriver)
	if !ok {
		return c.String(http.StatusBadRequest, "Bad context pass to handler")
	}

	return handler.processAsyncRequest(c)
}

func validateCommandValue(resource model.DeviceResource, reading interface{}, valueType string) (interface{}, error) {
	var err error
	castError := "failed to parse %v reading, %v"

	var val interface{}
	switch valueType {
	case common.ValueTypeObject:
		data, ok := reading.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(castError, resource.Name, "not a map")
		}
		val = data
	case common.ValueTypeBool:
		val, err = cast.ToBoolE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
	case common.ValueTypeString:
		val, err = cast.ToStringE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
	case common.ValueTypeUint8:
		val, err = cast.ToUint8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkUintValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeUint16:
		val, err = cast.ToUint16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkUintValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeUint32:
		val, err = cast.ToUint32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkUintValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeUint64:
		val, err = cast.ToUint64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkUintValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeInt8:
		val, err = cast.ToInt8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkIntValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeInt16:
		val, err = cast.ToInt16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkIntValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeInt32:
		val, err = cast.ToInt32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkIntValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeInt64:
		val, err = cast.ToInt64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkIntValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeFloat32:
		val, err = cast.ToFloat32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkFloatValueRange(valueType, val); err != nil {
			return nil, err
		}
	case common.ValueTypeFloat64:
		val, err = cast.ToFloat64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, resource.Name, err)
		}
		if err := checkFloatValueRange(valueType, val); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("return result fail, unsupported value type: %v", valueType)
	}

	return val, nil
}

func checkUintValueRange(valueType string, val interface{}) error {
	switch valueType {
	case common.ValueTypeUint8:
		_, ok := val.(uint8)
		if ok {
			return nil
		}
	case common.ValueTypeUint16:
		_, ok := val.(uint16)
		if ok {
			return nil
		}
	case common.ValueTypeUint32:
		_, ok := val.(uint32)
		if ok {
			return nil
		}
	case common.ValueTypeUint64:
		_, ok := val.(uint64)
		if ok {
			return nil
		}
	}
	return fmt.Errorf("value %v for %s type is out of range", val, valueType)
}

func checkIntValueRange(valueType string, val interface{}) error {
	switch valueType {
	case common.ValueTypeInt8:
		_, ok := val.(int8)
		if ok {
			return nil
		}
	case common.ValueTypeInt16:
		_, ok := val.(int16)
		if ok {
			return nil
		}
	case common.ValueTypeInt32:
		_, ok := val.(int32)
		if ok {
			return nil
		}
	case common.ValueTypeInt64:
		_, ok := val.(int64)
		if ok {
			return nil
		}
	}
	return fmt.Errorf("value %v for %s type is out of range", val, valueType)
}

func checkFloatValueRange(valueType string, val interface{}) error {
	switch valueType {
	case common.ValueTypeFloat32:
		valFloat := val.(float32)
		//if math.Abs(float64(valFloat)) >= math.SmallestNonzeroFloat32 && math.Abs(float64(valFloat)) <= math.MaxFloat32 {
		if math.Abs(float64(valFloat)) <= math.MaxFloat32 {
			return nil
		}
	case common.ValueTypeFloat64:
		valFloat := val.(float64)
		//if math.Abs(valFloat) >= math.SmallestNonzeroFloat64 && math.Abs(valFloat) <= math.MaxFloat64 {
		if math.Abs(valFloat) <= math.MaxFloat64 {
			return nil
		}
	}

	return fmt.Errorf("value %v for %s type is out of range", val, valueType)
}
