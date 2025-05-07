/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import "fmt"

type RoleResourcePermission struct {
	RoleName      string `json:"roleName,omitempty" codec:"roleName,omitempty"`
	ResourcesName string `json:"resourceName,omitempty" codec:"resourceName,omitempty"`
	Permission    string `json:"permission,omitempty" codec:"permission,omitempty"`
}

func (roleResourcePermission *RoleResourcePermission) TableName() string {
	return "hedge.role_resource_permission"
}
func (roleResourcePermission RoleResourcePermission) ToString() string {
	return fmt.Sprintf("roleName: %s\nresourceName: %s\npermission: %s", roleResourcePermission.RoleName, roleResourcePermission.ResourcesName, roleResourcePermission.Permission)
}
