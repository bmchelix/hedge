/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import "fmt"

type User struct {
	FullName       string `json:"fullName" codec:"fullName" binding:"required"`
	Email          string `json:"email" codec:"email"`
	KongUsername   string `json:"userId" gorm:"primary_key" codec:"kongUsername" binding:"required" validate:"max=200,matchRegex=^[a-zA-Z0-9][a-zA-Z0-9_-]*$"`
	ExternalUserId string `json:"externalUserId"`
	Status         string `json:"status" codec:"status"`
	CreatedOn      string `json:"createdOn" codec:"createdOn"`
	CreatedBy      string `json:"createdBy" codec:"createdBy"`
	ModifiedOn     string `json:"modifiedOn" codec:"modifiedOn"`
	ModifiedBy     string `json:"modifiedBy" codec:"modifiedBy"`
	Roles          []Role `gorm:"many2many:user_roles" json:"roles"`
}

func (user User) TableName() string {
	return "hedge.user"
}

func (user User) ToString() string {
	return fmt.Sprintf("full_name: %s\nemail: %s\nusername: %s\nstatus: %s", user.FullName, user.Email, user.KongUsername, user.Status)
}
