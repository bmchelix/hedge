/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type DeviceSummary struct {
	Id             string   `json:"id,omitempty"             codec:"id,omitempty"`
	Name           string   `json:"name"`
	Node           Node     `json:"node"`
	OperatingState string   `json:"operatingState,omitempty" codec:"operatingState,omitempty"`
	ProfileName    string   `json:"profileName,omitempty"    codec:"profileName,omitempty"`
	Labels         []string `json:"labels,omitempty"         codec:"labels,omitempty"`
	DeviceService  string   `json:"deviceService,omitempty"  codec:"deviceService,omitempty"`
	Location       Location `json:"location,omitempty"       codec:"location,omitempty"`
}

type Query struct {
	Filter Filter
	Page   Page
}

type Filter struct {
	Device   string
	Service  string
	Profile  string
	Labels   string
	EdgeNode string
	SortBy   string
	SortType string
	Present  bool `default:"false"`
}

type Page struct {
	Size   int `json:"PageSize"`
	Number int `json:"Page"`
	Total  int `json:"TotalCount"`
}

// NewDeviceSummary : Simple constructor, to be used from hedge-device-extn services
func NewDeviceSummary(name string) *DeviceSummary {
	deviceSummary := new(DeviceSummary)
	deviceSummary.Name = name
	return deviceSummary
}

func (dm *DeviceSummary) GetType() string {
	return "Device"
}

func (dm *DeviceSummary) GetId() string {
	return dm.Id
}

func (dm *DeviceSummary) GetName() string {
	return dm.Name
}
