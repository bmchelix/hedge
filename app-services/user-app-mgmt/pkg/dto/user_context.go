/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type LinkDetails struct {
	IsExternalUrl bool   `json:"isExternalUrl,omitempty" codec:"isExternalUrl,omitempty"` // External, Internal ie relative, or None, if external, add within iFrame
	LinkUri       string `json:"linkUri" codec:"linkUri"`
	LinkType      string `json:"linkType" codec:"linkType"`
	LinkId        string `json:"id" codec:"id"`
}

type Resource struct {
	Id                 int         `json:"id" codec:"Id"`
	Name               string      `json:"name" codec:"name" validate:"max=200,matchRegex=^[a-zA-Z0-9_\\-]+$"`
	DisplayName        string      `json:"displayName" codec:"displayName" validate:"max=200,matchRegex=^[a-zA-Z0-9_\\- ]+$"`
	AllowedPermissions string      `json:"allowedPermissions" codec:"allowedPermissions"`
	Tooltip            string      `json:"toolTip" codec:"toolTip"`
	IsTerminalItem     bool        `json:"isTerminalItem" codec:"isTerminalItem"` // true if leaf node and there are no further child nodes
	DisplaySequence    int         `json:"displaySequence,omitempty" codec:"displaySequence,omitempty"`
	LinkDetails        LinkDetails `json:"linkDetails,omitempty" codec:"linkDetails,omitempty"` // Blank if not a Terminal Node
	SubResources       []Resource  `json:"subResources,omitempty" codec:"subResources,omitempty"`
}

type UserContext struct {
	KongUsername    string      `json:"userId" codec:"kongUsername"`
	FullName        string      `json:"userName" codec:"userName"`
	ApplicationName string      `json:"applicationName" codec:"applicationName"` // eg Fleet Management
	LandingPage     LinkDetails `json:"landingPage,omitempty" codec:"landingPage,omitempty"`
	Resources       []Resource  `json:"menuItems,omitempty" codec:"menuItems,omitempty"`
}
