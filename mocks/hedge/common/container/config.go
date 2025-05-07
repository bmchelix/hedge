/*******************************************************************************
 * Copyright 2019 Dell Inc.
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

package container

import (
	bootstrapConfig "github.com/edgexfoundry/go-mod-bootstrap/v3/config"

	"github.com/edgexfoundry/go-mod-bootstrap/v3/di"
)

type ConfigurationStruct struct {
	//TODO: Remove in EdgeX 3.0 - Is needed now for backward compatability in 2.0
	RequireMessageBus bool
	Writable          WritableInfo
	Clients           map[string]bootstrapConfig.ClientInfo
	Databases         map[string]bootstrapConfig.Database
	Notifications     NotificationInfo
	Registry          bootstrapConfig.RegistryInfo
	Service           bootstrapConfig.ServiceInfo
	MessageBus        bootstrapConfig.MessageBusInfo
	SecretStore       bootstrapConfig.SecretStoreInfo
	UoM               UoM
}
type WritableInfo struct {
	LogLevel        string
	ProfileChange   ProfileChange
	UoM             WritableUoM
	InsecureSecrets bootstrapConfig.InsecureSecrets
	Telemetry       bootstrapConfig.TelemetryInfo
}
type NotificationInfo struct {
	Content           string
	Description       string
	Label             string
	PostDeviceChanges bool
	Sender            string
	Slug              string
}
type UoM struct {
	UoMFile string
}
type ProfileChange struct {
	StrictDeviceProfileChanges bool
	StrictDeviceProfileDeletes bool
}
type WritableUoM struct {
	Validation bool
}

// ConfigurationName contains the name of the metadata's config.ConfigurationStruct implementation in the DIC.
var ConfigurationName = di.TypeInstanceToName((*ConfigurationStruct)(nil))

// ConfigurationFrom helper function queries the DIC and returns metadata's config.ConfigurationStruct implementation.
func ConfigurationFrom(get di.Get) *ConfigurationStruct {
	return get(ConfigurationName).(*ConfigurationStruct)
}
