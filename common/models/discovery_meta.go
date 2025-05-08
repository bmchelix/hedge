/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type Discovery struct {
	Protocols map[string]string `json:"protocols"`
}

// type ProtocolProperties map[string]string
