package helpers

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
	"testing"

	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewModelEdgeStatusSubscriber(t *testing.T) {
	mockedDbClient := redisMock.MockMLDbInterface{}
	subscriber := NewModelEdgeStatusSubscriber(&mockedDbClient)

	if subscriber != nil && subscriber.dbClient != &mockedDbClient {
		t.Errorf("Expected dbClient to be set to the mockDbClient, but it was not.")
	}
	if subscriber == nil {
		t.Errorf("Expected ModelEdgeStatusSubscriber expected to be non-nil, but it was nil.")
	}
}

func TestModelEdgeStatusSubscriber_RecordModelSyncNodeStatus(t *testing.T) {
	t.Run("RecordModelSyncNodeStatus - Passed", func(t *testing.T) {
		mockedDbClient := redisMock.MockMLDbInterface{}
		mockedModelDeploymentStatus := ml_model.ModelDeploymentStatus{
			NodeId:               "node123",
			MLModelConfigName:    "config123",
			MLAlgorithm:          "algorithm123",
			ModelVersion:         1,
			DeploymentStatusCode: ml_model.ModelUnDeployed,
			Message:              "Sync successful",
		}
		mockedDbClient.On("GetDeploymentsByNode", mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{mockedModelDeploymentStatus}, nil)
		mockedDbClient.On("UpdateModelDeployment", mock.Anything).Return(nil)

		edgeStatus := &ModelEdgeStatusSubscriber{
			dbClient: &mockedDbClient,
		}

		mockedModelDeploymentStatusBytes, _ := util.CoerceType(mockedModelDeploymentStatus)
		got, got1 := edgeStatus.RecordModelSyncNodeStatus(
			u.AppFunctionContext,
			mockedModelDeploymentStatusBytes,
		)
		assert.False(t, got, "Expected 'got' to be false")
		assert.Nil(t, got1, "Expected 'got1' to be nil")
	})
	t.Run("RecordModelSyncNodeStatus - Failed (Invalid data type)", func(t *testing.T) {
		mockedDbClient := redisMock.MockMLDbInterface{}

		edgeStatus := &ModelEdgeStatusSubscriber{
			dbClient: &mockedDbClient,
		}

		data := []byte{}
		got, got1 := edgeStatus.RecordModelSyncNodeStatus(u.AppFunctionContext, data)
		assert.False(t, got, "Expected 'got' to be false")
		assert.EqualError(
			t,
			got1.(error),
			"function RecordModelSyncNodeStatus in pipeline 'erty-876trfv-dsdf', type received is not ml_model.ModelDeploymentStatus",
			"Expected error message mismatch",
		)
	})
	t.Run(
		"RecordModelSyncNodeStatus - Failed (GetDeploymentsByNode returns nil)",
		func(t *testing.T) {
			mockedDbClient := redisMock.MockMLDbInterface{}
			mockedDbClient.On("GetDeploymentsByNode", mock.Anything).Return(nil, nil)

			edgeStatus := &ModelEdgeStatusSubscriber{
				dbClient: &mockedDbClient,
			}

			mockedModelDeploymentStatus := ml_model.ModelDeploymentStatus{
				NodeId: "node123",
			}
			mockedModelDeploymentStatusBytes, _ := util.CoerceType(mockedModelDeploymentStatus)
			got, got1 := edgeStatus.RecordModelSyncNodeStatus(
				u.AppFunctionContext,
				mockedModelDeploymentStatusBytes,
			)
			assert.False(t, got, "Expected 'got' to be false")
			assert.Nil(t, got1, "Expected 'got1' to be nil")
		},
	)
	t.Run("RecordModelSyncNodeStatus - Failed (UpdateModelDeployment error)", func(t *testing.T) {
		mockedDbClient := redisMock.MockMLDbInterface{}
		mockedError := hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"mocked error",
		)
		mockedDbClient.On("GetDeploymentsByNode", mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{
				{
					NodeId: "node123",
				},
			}, nil)
		mockedDbClient.On("UpdateModelDeployment", mock.Anything).Return(mockedError)

		edgeStatus := &ModelEdgeStatusSubscriber{
			dbClient: &mockedDbClient,
		}

		mockedModelDeploymentStatus := ml_model.ModelDeploymentStatus{
			NodeId: "node123",
		}
		mockedModelDeploymentStatusBytes, _ := util.CoerceType(mockedModelDeploymentStatus)
		got, got1 := edgeStatus.RecordModelSyncNodeStatus(
			u.AppFunctionContext,
			mockedModelDeploymentStatusBytes,
		)
		assert.False(t, got, "Expected 'got' to be false")
		assert.Equal(t, mockedError, got1, "Expected 'got1' to match mocked error")
	})
}
