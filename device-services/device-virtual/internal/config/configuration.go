//
// Copyright (c) 2021 Intel Corporation
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

package config

import (
	"hedge/common/models"
	"errors"
)

// This file contains example of custom configuration that can be loaded from the service's configuration.toml
// and/or the Configuration Provider, aka Consul (if enabled).
// For more details see https://docs.edgexfoundry.org/2.0/microservices/device/Ch-DeviceServices/#custom-configuration

// Example structured custom configuration types. Must be wrapped with an outer struct with
// single element that matches the top level custom configuration element in your configuration.toml file,
// 'SimpleCustom' in this example.
type VirtualDeviceConfiguration struct {
	VirtualDeviceConfig VirtualDeviceConfig
	Hedge               models.HedgeSvcConfig
}

// SimpleCustomConfig is example of service's custom structured configuration that is specified in the service's
// configuration.toml file and Configuration Provider (aka Consul), if enabled.
type VirtualDeviceConfig struct {
	ServiceName         string
	DeviceNamePrefix    string
	MaxDiscoveredDevice int64
	Writable            VirtialDeviceWritable
}

// SimpleWritable defines the service's custom configuration writable section, i.e. can be updated from Consul
type VirtialDeviceWritable struct {
	DiscoverSleepDurationSecs int64
}

// UpdateFromRaw updates the service's full configuration from raw data received from
// the Service Provider.
func (sw *VirtualDeviceConfiguration) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*VirtualDeviceConfiguration)
	if !ok {
		return false //errors.New("unable to cast raw config to type 'ServiceConfig'")
	}

	*sw = *configuration

	return true
}

// Validate ensures your custom configuration has proper values.
// Example of validating the sample custom configuration
func (scc *VirtualDeviceConfig) Validate() error {
	if len(scc.DeviceNamePrefix) == 0 {
		return errors.New("VirtualDeviceConfig.DeviceNamePrefix configuration setting can not be blank")
	}

	if scc.MaxDiscoveredDevice < 0 {
		return errors.New("VirtualDeviceConfig.MaxDiscoveredDevice configuration setting can not be less than 0")
	}

	if scc.Writable.DiscoverSleepDurationSecs < 10 {
		return errors.New("VirtualDeviceConfig.Writable.DiscoverSleepDurationSecs configuration setting must be 10 or greater")
	}

	return nil
}
