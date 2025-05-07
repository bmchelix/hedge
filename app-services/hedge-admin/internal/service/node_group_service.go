/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-admin/db"
	models2 "hedge/app-services/hedge-admin/models"
	db2 "hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"strings"
)

type NodeGroupService struct {
	service  interfaces.ApplicationService
	dbClient db.RedisDbClient
}

func NewNodeGroupService(service interfaces.ApplicationService) *NodeGroupService {
	nodeGroupSvc := new(NodeGroupService)
	nodeGroupSvc.service = service
	nodeGroupSvc.dbClient = db.NewDBClient(service)
	return nodeGroupSvc
}

func (ng *NodeGroupService) SaveNode(node *models.Node) (string, hedgeErrors.HedgeError) {
	if node.HostName == "" {
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Hostname cannot be empty")
	}
	if node.NodeId == "" {
		node.NodeId = node.HostName
	}
	key, err := ng.dbClient.SaveNode(node)
	if err != nil {
		ng.service.LoggingClient().Errorf("Failed to save node %s, err: %v", node.NodeId, err)
		return key, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Error saving node %s", node.NodeId))
	}

	return key, nil
}

func (ng *NodeGroupService) GetNode(nodeName string) (*models.Node, hedgeErrors.HedgeError) {
	node, err := ng.dbClient.GetNode(nodeName)
	if err != nil {
		ng.service.LoggingClient().Errorf("Failed to get node %s, err: %v", nodeName, err)
		return nil, err
	}
	return buildNodeURLs(node), nil
}

func (ng *NodeGroupService) DeleteNode(nodeId string) hedgeErrors.HedgeError {
	keyFieldHashkeysToRemoveMembers, err := ng.getNodeGroupHashKeysMatchingANode("", nodeId)
	if err != nil {
		ng.service.LoggingClient().Errorf("Failed to delete node %s, err: %v", nodeId, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error deleting node %s", nodeId))
	}

	return ng.dbClient.DeleteNode(nodeId, keyFieldHashkeysToRemoveMembers)
}

func buildNodeURLs(node *models.Node) *models.Node {

	if node.RuleEndPoint == "" {
		if node.IsRemoteHost {
			node.RuleEndPoint = fmt.Sprintf("/hedge/api/v3/rules/%s/", node.NodeId)
		} else {
			//use container name for local(non-remote) machines since the external port will not be open for it
			node.RuleEndPoint = "/hedge/api/v3/rules/edgex-kuiper/"
		}
	}
	if node.WorkFlowEndPoint == "" {
		if node.IsRemoteHost {
			node.WorkFlowEndPoint = fmt.Sprintf("/hedge/hedge-node-red/%s/", node.NodeId)
		} else {
			node.WorkFlowEndPoint = "/hedge/hedge-node-red/hedge-node-red/"
		}
	}
	return node
}

func (ng *NodeGroupService) GetNodes() ([]models.Node, hedgeErrors.HedgeError) {
	nodes, err := ng.dbClient.GetAllNodes()
	for i, node := range nodes {
		nodes[i] = *buildNodeURLs(&node)
	}
	return nodes, err
}

func (ng *NodeGroupService) SaveNodeGroup(parentGroupName string, nodeGroup *models.NodeGroup) (string, hedgeErrors.HedgeError) {
	return ng.dbClient.SaveNodeGroup(parentGroupName, toDBNodeGroup(nodeGroup))
}

func (ng *NodeGroupService) GetNodeGroups(parentGroupName string) ([]models.NodeGroup, hedgeErrors.HedgeError) {
	// 1. Find the root hashkey
	// 2. Using hashkey, find all nodeGroups
	// 3. For each nodeGroup found above, get nodeGroupDetails and call #2 recursively
	parentGroupHashKey, err := ng.dbClient.FindNodeKey(parentGroupName)
	if err != nil {
		return nil, err
	}
	nodeGroups, err := ng.getNodeGroups(parentGroupHashKey)
	if err == nil && len(nodeGroups) == 0 {
		ng.service.LoggingClient().Errorf("Failed to get node group by parentGroupHaskKey: %s", parentGroupHashKey)
		return nodeGroups, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Node groups not found by parent group name %s", parentGroupName))
	}
	return nodeGroups, err
}

func (ng *NodeGroupService) DeleteNodeGroup(groupName string) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error deleting node group %s", groupName)

	nodeGroupKey, err := ng.dbClient.FindNodeKey(groupName)
	if err != nil {
		return err
	}
	if nodeGroupKey == "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("%s: %s", errorMessage, "Node key not found"))
	}
	members, err := ng.dbClient.GetDBNodeGroupMembers(nodeGroupKey)
	if err != nil {
		return err
	}
	if len(members) > 0 {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, "Group has sub-groups, so cannot be deleted"))
	}
	parentGroup := strings.ReplaceAll(nodeGroupKey, ":"+groupName, "")
	err = ng.dbClient.DeleteNodeGroup(parentGroup, groupName)
	if err != nil {
		ng.service.LoggingClient().Errorf("Failed to delete node group %s, err: %v", groupName, err)
		return err
	}
	ng.service.LoggingClient().Debugf("ParentGroup:%s from which node:%s will be deleted, members: %v", nodeGroupKey, groupName, members)
	return nil
}

func (ng *NodeGroupService) AddNodeToDefaultGroup(groupName string, groupDisplayName string, node *models.Node) hedgeErrors.HedgeError {
	//First create the parent group if it doesn't exist
	_, err := ng.GetNodeGroups(groupName)
	if err != nil && err.IsErrorType(hedgeErrors.ErrorTypeNotFound) {
		// Create the default group
		parentDefaultGroup := models.NodeGroup{
			Name:        groupName,
			DisplayName: groupDisplayName,
		}
		_, err := ng.SaveNodeGroup("", &parentDefaultGroup)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	//Now add the container for the node as a child of the parent just created
	nodeContainer := models.NodeGroup{
		Name:        node.HostName + "_Container",
		DisplayName: node.HostName,
		Node:        node,
	}
	_, err = ng.SaveNodeGroup(groupName, &nodeContainer)

	return err
}

func (ng *NodeGroupService) getNodeGroups(hashKey string) ([]models.NodeGroup, hedgeErrors.HedgeError) {
	nodeGroups := make([]models.NodeGroup, 0)
	dbNodeGroups, err := ng.dbClient.GetDBNodeGroupMembers(hashKey)
	if err != nil {
		return nil, err
	}

	for _, dbNodeGrp := range dbNodeGroups {
		//dbNodeGrp, err := ng.dbClient.GetNodeGroup(dbNodeGroup.Name)

		nodeGroup := toNodeGroup(dbNodeGrp)
		// Get the Node if there is one
		if dbNodeGrp.NodeId != "" {
			//node, err := ng.dbClient.GetNode(dbNodeGrp.NodeName)
			node, err := ng.GetNode(dbNodeGrp.NodeId)
			if err != nil {
				ng.service.LoggingClient().Warnf("Error getting Node for nodeName: %s, Error: %v", dbNodeGrp.NodeId, err)
				continue
			}
			nodeGroup.Node = node
		}
		// Now need to get the children and attach the same to childNodes
		nodeGroup.ChildNodeGroups, err = ng.getNodeGroups(hashKey + ":" + nodeGroup.Name)
		if err != nil {
			ng.service.LoggingClient().Warnf("Error getting NodeGroups for hashkey:%s, Error: %v", hashKey, err)
			continue
		}

		nodeGroups = append(nodeGroups, nodeGroup)

	}

	return nodeGroups, nil
}

func toNodeGroup(dbNodeGroup models.DBNodeGroup) models.NodeGroup {
	nodeGroup := models.NodeGroup{
		Name:        dbNodeGroup.Name,
		DisplayName: dbNodeGroup.DisplayName,
	}
	return nodeGroup
}

func toDBNodeGroup(nodeGroup *models.NodeGroup) *models.DBNodeGroup {
	dbNodeGroup := models.DBNodeGroup{
		Name:        nodeGroup.Name,
		DisplayName: nodeGroup.DisplayName,
	}
	if nodeGroup.Node != nil {
		// Check if this is required since now hostName is moved to node itself
		dbNodeGroup.NodeId = nodeGroup.Node.NodeId
	}
	return &dbNodeGroup

}

func (ng *NodeGroupService) getNodeGroupHashKeysMatchingANode(hashKey string, nodeId string) ([]models2.KeyFieldTuple, hedgeErrors.HedgeError) {
	keyFieldTuples := make([]models2.KeyFieldTuple, 0)
	if hashKey == "" {
		hashKey = db2.NodeGroup
	}
	dbNodeGroups, err := ng.dbClient.GetDBNodeGroupMembers(hashKey)
	if err != nil || len(dbNodeGroups) == 0 {
		return keyFieldTuples, err
	}

	for _, dbNodeGrp := range dbNodeGroups {
		//dbNodeGrp, err := ng.dbClient.GetNodeGroup(dbNodeGroup.Name)

		nodeGroup := toNodeGroup(dbNodeGrp)
		// Get the Node if there is one
		if dbNodeGrp.NodeId != "" && nodeId == dbNodeGrp.NodeId {
			_, err := ng.GetNode(dbNodeGrp.NodeId)
			if err == nil {
				keyFieldTuple := models2.KeyFieldTuple{
					Key:   hashKey,
					Field: dbNodeGrp.Name,
				}
				keyFieldTuples = append(keyFieldTuples, keyFieldTuple)
				ng.service.LoggingClient().Infof("nodeGroup key & field marked for delete: %v", keyFieldTuples)
			} else {
				continue
			}

			//nodeGroup.Node = node
		}
		// Now need to get the children and attach the same to childNodes
		tuples, err := ng.getNodeGroupHashKeysMatchingANode(hashKey+":"+nodeGroup.Name, nodeId)
		if tuples != nil && len(tuples) > 0 {
			for _, tuple := range tuples {
				keyFieldTuples = append(keyFieldTuples, tuple)
			}
		}
		if err != nil {
			ng.service.LoggingClient().Warnf("Error getting NodeGroups for hashkey:%s, error: %v", hashKey, err)
			continue
		}

		//nodeGroups = append(nodeGroups, nodeGroup)

	}

	return keyFieldTuples, nil
}
