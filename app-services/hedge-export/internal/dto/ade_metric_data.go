/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type AdeMetric struct {
	Labels  Labels   `json:"labels"`
	Samples []Sample `json:"samples"`
}

type Labels struct {
	MetricName   string `json:"metricName"`
	Hostname     string `json:"hostname"`
	EntityTypeId string `json:"entityTypeId,omitempty" codec:"entityTypeId,omitempty"`
	EntityName   string `json:"entityName,omitempty" codec:"entityName,omitempty"`
	EntityId     string `json:"entityId,omitempty" codec:"entityId,omitempty"`
	HostType     string `json:"hostType,omitempty" codec:"hostType,omitempty"`
	IsKPI        bool   `json:"isKpi,omitempty" codec:"isKpi,omitempty"`
	Unit         string `json:"unit,omitempty" codec:"unit,omitempty"`
	Source       string `json:"source,omitempty" codec:"source,omitempty"`
}

type Sample struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"` // In milliseconds
}
