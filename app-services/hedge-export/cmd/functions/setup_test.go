package functions

import (
	"hedge/common/dto"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"time"
)

var (
	u                             *utils.HedgeMockUtils
	ctx                           interfaces.AppFunctionContext
	mockLogger                    logger.LoggingClient
	testMetricsData               dto.Metrics
	testMetricsDataWithoutSamples dto.Metrics
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{"DataStore_Provider": "ADE"})
	ctx = u.AppFunctionContext
	mockLogger = u.AppService.LoggingClient()

	testMetricsData = dto.Metrics{
		IsCompressed: false,
		MetricGroup: dto.MetricGroup{
			Tags: map[string]any{
				"environment": "production",
				"region":      "us-east-1",
			},
			Samples: []dto.Data{
				{
					Name:      "cpu_usage",
					TimeStamp: time.Now().UnixNano(),
					Value:     "75.5",
					ValueType: "float",
				},
				{
					Name:      "memory_usage",
					TimeStamp: time.Now().UnixNano(),
					Value:     "2048",
					ValueType: "integer",
				},
			},
		},
	}
	testMetricsDataWithoutSamples = dto.Metrics{
		IsCompressed: false,
		MetricGroup: dto.MetricGroup{
			Tags: map[string]any{
				"environment": "production",
				"region":      "us-east-1",
			},
		},
	}
}
