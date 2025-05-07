/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type ContentData struct {
	NodeType    string   `json:"nodeType,omitempty"`
	ContentDir  []string `json:"contentDir,omitempty"`
	TargetNodes []string `json:"targetNodes,omitempty"`
}

type KeyFieldTuple struct {
	Key   string
	Field string
}

type VaultSecretData struct {
	Key   string
	Value string
}
