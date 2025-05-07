/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type DeviceExtension struct {
	Field string `json:"field,omitempty" codec:"field,omitempty"`
	// Consider adding a Type field in future so UI can render the entry field correctly
	// For now, all are Text fields
	Default     string `json:"default,omitempty" codec:"default,omitempty"`
	IsMandatory bool   `json:"isMandatory,omitempty" codec:"isMandatory,omitempty"`
}

type ProfileSummary struct {
	Name                 string   `json:"name" codec:"name"`
	Description          string   `json:"description,omitempty" codec:"description,omitempty"`
	MetricNames          []string `json:"metricNames" codec:"metricNames"`
	DeviceAttributes     []string `json:"deviceAttributes" codec:"deviceAttributes"`
	ContextualAttributes []string `json:"contextualAttributes" codec:"contextualAttributes"`
}
