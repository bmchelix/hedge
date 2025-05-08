/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	"encoding/json"
	"fmt"

	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type ModelEdgeStatusSubscriber struct {
	dbClient redis.MLDbInterface
}

func NewModelEdgeStatusSubscriber(dbClient redis.MLDbInterface) *ModelEdgeStatusSubscriber {
	mlModelDownloadSubs := new(ModelEdgeStatusSubscriber)
	mlModelDownloadSubs.dbClient = dbClient
	return mlModelDownloadSubs
}

// The below function is part of edgex pipeline API
func (edgeStatus *ModelEdgeStatusSubscriber) RecordModelSyncNodeStatus(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {

	// ModelDeploymentStatus was converted to []byte in previous step
	jsonBytes, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf(
			"function RecordModelSyncNodeStatus in pipeline '%s', "+
				"type received is not ml_model.ModelDeploymentStatus",
			ctx.PipelineId())
	}

	var status ml_model.ModelDeploymentStatus
	err := json.Unmarshal(jsonBytes, &status)
	if err != nil {
		ctx.LoggingClient().Errorf("Failed to unmarshal JSON data: %v, error: %v", string(jsonBytes), err)
		return false, fmt.Errorf(
			"function RecordModelSyncNodeStatus in pipeline '%s', "+
				"type received is not ml_model.ModelDeploymentStatus",
			ctx.PipelineId())
	}

	ctx.LoggingClient().Infof(
		"model deployment status received for node: %s algorithm: %s status: %s",
		status.NodeId, status.MLAlgorithm, status.DeploymentStatus,
	)

	nodeDeployments, err := edgeStatus.dbClient.GetDeploymentsByNode(status.NodeId)
	if err != nil || nodeDeployments == nil || len(nodeDeployments) == 0 {
		return false, err
	}
	for _, deployment := range nodeDeployments {
		if deployment.MLModelConfigName == status.MLModelConfigName &&
			deployment.MLAlgorithm == status.MLAlgorithm &&
			deployment.ModelVersion == status.ModelVersion {
			// Ignore updates when the command was just to sync MLEvent changes
			if status.DeploymentStatusCode != ml_model.ModelEventConfigSyncSuccess &&
				status.DeploymentStatusCode != ml_model.ModelEventConfigSyncFailed {
				deployment.DeploymentStatusCode = status.DeploymentStatusCode
				deployment.DeploymentStatus = status.DeploymentStatusCode.String()
			}
			deployment.Message = status.Message
			err := edgeStatus.dbClient.UpdateModelDeployment(deployment)
			if err != nil {
				ctx.LoggingClient().Errorf("Failed to update node deployment status, error %v", err)
				return false, err
			} else {
				continue
			}
		}
	}

	return false, nil
}
