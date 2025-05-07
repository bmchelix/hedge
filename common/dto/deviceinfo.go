/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import (
	models2 "hedge/common/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
)

type DeviceObject struct {
	ApiVersion           string                 `json:"apiVersion" codec:"apiVersion"`
	Device               dtos.Device            `json:"device" codec:"device"`
	Node                 models2.Node           `json:"node" codec:"node"`
	Associations         []AssociationNode      `json:"associations,omitempty" codec:"associations,omitempty"`
	Extensions           []DeviceExtResp        `json:"extensions,omitempty" codec:"extensions,omitempty"`
	ContextualAttributes map[string]interface{} `json:"contextualAttributes,omitempty" codec:"contextualAttributes,omitempty"`
}

type DeviceArr struct {
	ApiVersion string        `json:"apiVersion" codec:"apiVersion"`
	TotalCount int           `json:"totalCount"`
	Devices    []dtos.Device `json:"devices"`
}
