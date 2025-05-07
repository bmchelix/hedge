/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

// NodeGroup provides a logical way to group the Nodes, It enables node-selector in the UI
type NodeGroup struct {
	// Node may belong to multiple groups, so can't have groupName in here
	// GroupName string `json:"groupCategory"`
	Name            string      `json:"name"` // Name of the group, is also key field
	DisplayName     string      `json:"displayName"`
	Node            *Node       `json:"node,omitempty"`
	ChildNodeGroups []NodeGroup `json:"childNodeGroups,omitempty"`
}

type Node struct {
	// NodeId is the actual node IP/domain name that cane be used to URLs to build the actual URL
	NodeId string `json:"nodeId"` // Change this to nodeId, By default we make it as hostname if Id not provided
	// hostName can change, nodeId cannot change, use hostName instead of nodeId in UI
	HostName     string `json:"hostName"`
	IsRemoteHost bool   `json:"isRemoteHost", default:false`
	Name         string `json:"name,omitempty"`
	// ruleEndPoint: /hedge/api/v3/rules/{hostName}
	RuleEndPoint string `json:"ruleEndPoint"`
	// WorkflowEndPoint: /hedge/hedge-node-red/{hostName}
	WorkFlowEndPoint string `json:"workflowEndPoint"`
}

// Storage Structure, the key that we create to store takes care of hierarchy
type DBNodeGroup struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	NodeId      string `json:"nodeId,omitempty"`
}
