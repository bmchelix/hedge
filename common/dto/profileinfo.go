/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
)

type ProfileObject struct {
	ApiVersion       string             `json:"apiVersion" codec:"apiVersion"`
	Profile          dtos.DeviceProfile `json:"profile,omitempty" codec:"profile"`
	ProfileExtension                    // Embedded ProfileExtension
}

type ProfileExtension struct {
	DeviceAttributes     []DeviceExtension   `json:"deviceAttributes,omitempty" codec:"deviceAttributes,omitempty"`
	ContextualAttributes []string            `json:"contextualAttributes,omitempty" codec:"contextualAttributes,omitempty"`
	DownsamplingConfig   *DownsamplingConfig `json:"downsamplingConfig,omitempty" codec:"downsamplingConfig,omitempty"`
}

type Aggregate struct {
	FunctionName         string `json:"functionName,omitempty" codec:"functionName,omitempty"`
	GroupBy              string `json:"groupBy,omitempty" codec:"groupBy,omitempty"`
	SamplingIntervalSecs int64  `json:"samplingIntervalSecs,omitempty" codec:"samplingIntervalSecs,omitempty"`
}

type DownsamplingConfig struct {
	DefaultDataCollectionIntervalSecs int64       `json:"defaultDataCollectionIntervalSecs,omitempty" codec:"defaultDataCollectionIntervalSecs,omitempty"`
	DefaultDownsamplingIntervalSecs   int64       `json:"defaultDownsamplingIntervalSecs,omitempty" codec:"defaultDownsamplingIntervalSecs,omitempty"`
	Aggregates                        []Aggregate `json:"aggregates,omitempty" codec:"aggregates,omitempty"`
}

type ProfileArray struct {
	ApiVersion string               `json:"apiVersion" codec:"apiVersion"`
	TotalCount int                  `json:"totalCount"`
	Profiles   []dtos.DeviceProfile `json:"profiles" codec:"profile"`
}

func (p ProfileObject) ProfileIsEmpty() bool {
	if p.Profile.Id != "" || p.Profile.Name != "" || len(p.Profile.Labels) != 0 || p.Profile.Manufacturer != "" || len(p.Profile.DeviceResources) != 0 || len(p.Profile.DeviceCommands) != 0 || p.Profile.Model != "" || p.Profile.Description != "" {
		return false
	}
	return true
}
