/*******************************************************************************
 * Copyright 2018 Dell Inc.
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

package db

import (
	"errors"
	"time"
)

const (

	// Metadata
	Device        = "md|dv"
	DeviceProfile = "md|dp"
	DeviceService = "md|ds"

	// BMC hedge custom constants
	Association               = Device + "|hx:ass" //hx short for Helix-edge eXtension
	DeviceExt                 = Device + "|hx:ext"
	ProfileExt                = DeviceProfile + "|hx:ext"
	DeviceServiceExt          = DeviceService + "|hx:ext"
	ProfileDevicesExt         = DeviceExt + ":profile:name"
	DeviceContextualData      = Device + "|hx:ctx"
	ProfileContextualData     = DeviceProfile + "|hx:ctx"
	ProfileAttr               = DeviceProfile + "|hx:attr"
	ProfileDownsamplingConfig = DeviceProfile + "|hx:dconf"
	NodeRawDataConfig         = "hx:nrdconf"
	NodeGroup                 = "hx:nodeGroup"
	Node                      = "hx:node"
	DTwinScene                = "hx:dt:sc"
	DTwinImg                  = "hx:dt:img"

	// ML related redis storage keys
	MLAlgorithm    = "hx:ml:algo"
	MLModelConfig  = "hx:ml:mcfg"
	MLTrainingJob  = "hx:ml:trgjob"
	MLTrainedModel = "hx:ml:model"
	MLDeployment   = "hx:ml:depstat"
	MLEventConfig  = "hx:ml:evCfg"

	// Metric Export Data, Old: metricExportConfig
	ServiceConfig      = "hx:cfg"
	MetricExportConfig = ServiceConfig + ":mxp"
	MetricCounter      = Node + ":mc"

	// DigitalTwin Simulation
	TwinDefinition       = "hx:dt:def"
	SimulationDefinition = "hx:dt:sim"

	//Event and commands
	OTEvent       = "hx:ev"
	OTRemediation = "hx:rmdy"

	// Notification
	Notification = "notification"
	Subscription = "subscription"
	Transmission = "transmission"

	// Location
	Location = "location"
)

var (
	ErrNotFound            = errors.New("item not found")
	ErrUnsupportedDatabase = errors.New("unsupported database type")
	ErrInvalidObjectId     = errors.New("invalid object ID")
	ErrNotUnique           = errors.New("resource already exists")
	ErrCommandStillInUse   = errors.New("command is still in use by device profiles")
	ErrSlugEmpty           = errors.New("slug is nil or empty")
	ErrNameEmpty           = errors.New("name is required")
	ErrInternal            = errors.New("internal error")

	// ErrNotImplemented BMC hedge custom error message
	ErrNotImplemented   = errors.New("not implemented yet")
	ErrMaxLimitExceeded = errors.New("maximum allowed limit exceeded for the entity")
)

type Configuration struct {
	DbType       string
	Host         string
	Port         int
	Timeout      int
	DatabaseName string
	Username     string
	Password     string
	BatchSize    int
}

func MakeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
