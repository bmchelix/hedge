/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import "hedge/common/errors"

type NodeReqData struct {
	Flows    []ExportNodeData
	Rules    []ExportNodeData
	Profiles []string
}

type NodeData struct {
	ToNodeId []string
	Flows    []string
	Rules    []string
	Profiles []string
}

type Flow struct {
	FromNodeId string
	ToNodeId   []string
	Selected   bool
	Data       map[string]interface{}
}

type Rule struct {
	FromNodeId string
	ToNodeId   []string
	Name       []string
	Data       RuleData
}

type RuleData struct {
	Id      string        `json:"id"`
	Sql     string        `json:"sql"`
	Actions []interface{} `json:"actions"`
	Options interface{}   `json:"options"`
}

type ExportImportIndex struct {
	Imports  NodeData
	FilePath string
	Error    errors.HedgeError
}

type ExportImportFlow struct {
	Flows          []Flow
	FilePath       string
	FlowsIndexData []map[string]interface{}
	Errors         []NodeError `json:"errors"`
}

type ExportImportRules struct {
	Rules          []Rule
	FilePath       string
	RulesIndexData []map[string]interface{}
	Errors         []NodeError `json:"errors"`
}

type NodeError struct {
	NodeID string              `json:"nodeId"`
	Errors []errors.HedgeError `json:"errors"`
}

type ExportImportProfiles struct {
	Profiles          []ProfileObject
	FilePath          string
	ProfilesIndexData []string
	Errors            []ProfileError `json:"errors"`
}

type ProfileError struct {
	ProfileID string              `json:"profileId"`
	Errors    []errors.HedgeError `json:"errors"`
}

type ExportNodeData struct {
	NodeID string   `json:"nodeId" codec:"nodeId"`
	Host   string   `json:"host" codec:"host"`
	Ids    []string `json:"Ids" codec:"Ids"`
}

type ExportImportData struct {
	IndexData    ExportImportIndex
	FlowsData    ExportImportFlow
	RulesData    ExportImportRules
	ProfilesData ExportImportProfiles
}
