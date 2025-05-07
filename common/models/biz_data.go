/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type ExternalBusinessData struct {
	DeviceName string
	BizData    map[string]interface{}
}
