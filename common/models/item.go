/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

// An interface that needs to be implemented by all items (Device, Person) that participates in a association
type Item interface {
	GetType() string
	GetId() string
	GetName() string
}
