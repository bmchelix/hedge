/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import (
	"fmt"
)

type Role struct {
	Name                string      `json:"name" gorm:"primary_key" form:"name" binding:"required" validate:"max=200,matchRegex=^[a-zA-Z0-9_\\-]+$"`
	Description         string      `json:"description" form:"description"`
	RoleType            string      `json:"roleType" form:"roleType" validate:"max=200,matchRegex=^[a-zA-Z0-9_\\-]+$"`
	DefaultResourceName string      `json:"defaultResourceName"`
	CreatedOn           string      `json:"createdOn" codec:"createdOn"`
	CreatedBy           string      `json:"createdBy" codec:"createdBy"`
	ModifiedOn          string      `json:"modifiedOn" codec:"modifiedOn"`
	ModifiedBy          string      `json:"modifiedBy" codec:"modifiedBy"`
	Resources           []Resources `gorm:"many2many:role_resource_permission" json:"resources" validate:"dive"`
}

func (role Role) TableName() string {
	return "hedge.role"
}

func (role Role) ToString() string {
	return fmt.Sprintf("name: %s\ndescription: %s\nroleType: %s\ndefaultResourceName: %s", role.Name, role.Description, role.RoleType, role.DefaultResourceName)
}
