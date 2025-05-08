/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

type AdeUser struct {
	Auth_type         string `json:"auth_type"`
	Created_date_time string `json:"created_date_time"`
	Principal_id      string `json:"principal_id"`
	Status            string `json:"status"`
	Tenant_id         string `json:"tenant_id"`
	Type              string `json:"type"`
	User_id           string `json:"user_id"`
}

type AdeUserArray struct {
	Records []AdeUser `json:"records"`
}

type AdeGroup struct {
	Group_id          string `json:"group_id"`
	Group_source_type string `json:"group_source_type"`
	Name              string `json:"name"`
	System_object     bool   `json:"system_object"`
}

type AdeGroupArray struct {
	Records []AdeGroup `json:"records"`
}

type AdeUserOp struct {
	Id string `json:"id"`
	Op string `json:"op"` // add or remove
}

type GroupUserOp struct {
	Users []AdeUserOp `json:"users"`
}

type Device struct {
	UnrestrictedAccess bool     `json:"unrestrictedAccess"`
	RbacObjects        []string `json:"rbacObjects"`
}

type AllowedObjects struct {
	Device Device `json:"DEVICE"`
}

type AuthProfile struct {
	Name           string         `json:"name"`
	UserGroups     string         `json:"userGroups"`
	AllowedObjects AllowedObjects `json:"allowedObjects"`
}

type UserSearch struct {
	Filters []UserFields `json:"filters"`
}

type UserFields struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}
