/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import "fmt"

type UserRole struct {
	UserKongUsername string `json:"userId,omitempty"`
	RoleName         string `json:"role,omitempty"`
}

func (userRole *UserRole) TableName() string {
	return "hedge.user_roles"
}
func (userRole UserRole) ToString() string {
	return fmt.Sprintf("userKongUsername: %s\nroleName: %s\n", userRole.UserKongUsername, userRole.RoleName)
}
