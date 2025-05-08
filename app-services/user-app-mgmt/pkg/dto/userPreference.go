/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import "fmt"

type UserPreference struct {
	KongUsername string `json:"userId" gorm:"primary_key" codec:"kongUsername"`
	ResourceName string `json:"homePage" codec:"homePage"`
	CreatedOn    string `json:"createdOn" codec:"createdOn"`
	CreatedBy    string `json:"createdBy" codec:"createdBy"`
	ModifiedOn   string `json:"modifiedOn" codec:"modifiedOn"`
	ModifiedBy   string `json:"modifiedBy" codec:"modifiedBy"`
}

func (userPreference UserPreference) TableName() string {
	return "hedge.user_preference"
}

func (userPreference UserPreference) ToString() string {
	return fmt.Sprintf("userId: %s\nresourceName: %s", userPreference.KongUsername, userPreference.ResourceName)
}
