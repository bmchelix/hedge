/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type DeviceService struct {
	ApiVersion string  `json:"apiVersion"`
	Service    Service `json:"service"`
}

type Service struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AdminState  string   `json:"adminState"`
	Labels      []string `json:"labels"`
	BaseAddress string   `json:"baseAddress"`
}

type ServicesArray struct {
	ApiVersion string    `json:"apiVersion"`
	TotalCount int       `json:"totalCount"`
	Services   []Service `json:"services"`
}

type DevServArray []struct {
	DeviceService DeviceService
}
