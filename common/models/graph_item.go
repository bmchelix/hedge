/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

// The structure that holds the association data in a hierarchical way
type GraphItem struct {
	Item         Item   `json:"item,omitempty" codec:"item,omitempty"`
	Associations []Item `json:"associations,omitempty" codec:"associations,omitempty"`
}
