/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import (
	"fmt"
)

type Resources struct {
	Id                 int    `json:"id" form:"Id"`
	Name               string `json:"name" gorm:"primary_key" form:"Name" binding:"required"`
	DisplayName        string `json:"displayName" form:"displayName" codec:"displayName"`
	Uri                string `json:"uri" form:"Uri" binding:"required" codec:"uri"`
	LinkType           string `json:"linkType" codec:"linkType"`
	UiId               string `json:"uiId" codec:"uiId"`
	Active             bool   `json:"active" form:"Active"`
	ParentResource     string `json:"parentResource,omitempty" gorm:"foreignKey:Name" form:"parentResource"`
	AllowedPermissions string `json:"allowedPermissions" codec:"allowedPermissions"`
	CreatedOn          string `json:"createdOn" codec:"createdOn"`
	CreatedBy          string `json:"createdBy" codec:"createdBy"`
	ModifiedOn         string `json:"modifiedOn" codec:"modifiedOn"`
	ModifiedBy         string `json:"modifiedBy" codec:"modifiedBy"`
}

func (resources *Resources) TableName() string {
	return "hedge.resources"
}

func (resources Resources) ToString() string {
	return fmt.Sprintf("id: %d\nname: %s\ndisplayName: %s\nuri: %s\nlinkType: %s\nuiId: %s\nactive: %t\nparentResource: %s\nallowedPermissions: %s", resources.Id, resources.Name, resources.DisplayName, resources.Uri, resources.LinkType, resources.UiId, resources.Active, resources.ParentResource, resources.AllowedPermissions)
}
