package functions

import (
	"hedge/common/models"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
)

var (
	u              *utils.HedgeMockUtils
	testEventData  models.HedgeEvent
	testEventData1 models.HedgeEvent
	testError      = errors.New("dummy error")
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{"ElasticURL": "", "ElasticUser": "User", "ElasticPassword": "pass"})

	testEventData = models.HedgeEvent{
		Id:             "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
		Status:         "Open",
		Thresholds:     map[string]interface{}{"Threshold": float64(0)},
		ActualValues:   map[string]interface{}{"ActualValue": float64(0)},
		AdditionalData: map[string]string{"key": "value"},
		CorrelationId:  "123456",
		Remediations: []models.Remediation{
			{
				Id:      "123",
				Status:  "OK",
				Type:    "type",
				Summary: "summary",
			},
		},
		IsNewEvent:       false,
		IsRemediationTxn: false,
	}

	testEventData1 = models.HedgeEvent{
		Id:               "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b1",
		Status:           "Closed",
		IsNewEvent:       true,
		CorrelationId:    "",
		Priority:         "MEDIUM",
		Remediations:     []models.Remediation{{Id: "123", Type: "type", Summary: "summary", Status: "OK"}},
		IsRemediationTxn: true,
	}
}
