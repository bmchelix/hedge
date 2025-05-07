/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type SQLMetaData struct {
	DeviceAlias  string   `json:"deviceAlias,omitempty" codec:"deviceAlias,omitempty"`
	ProfileAlias string   `json:"profileAlias,omitempty" codec:"profileAlias,omitempty"`
	ProfileName  string   `json:"profileName,omitempty" codec:"profileName,omitempty"`
	MetricNames  []string `json:"metricName,omitempty" codec:"metricName,omitempty"`
}
