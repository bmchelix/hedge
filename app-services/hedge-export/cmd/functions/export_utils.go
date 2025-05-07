package functions

import (
	"encoding/json"

	"hedge/common/dto"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
)

func (bm *MetricExportService) convertToMetrics(
	ctx interfaces.AppFunctionContext, data interface{},
) interface{} {

	lc := ctx.LoggingClient()

	// convert to bytes
	jsonBytes, err := util.CoerceType(data)
	if err != nil {
		lc.Errorf("Returning from convertToMetrics, error: %s", err.Error())
		return nil
	}

	// Try to obtain Metrics data
	var metrics dto.Metrics
	err = json.Unmarshal(jsonBytes, &metrics)
	if err == nil && metrics.MetricGroup.Samples != nil {
		lc.Debugf("Returning from convertToMetrics, metrics: %s", metrics)
		return metrics
	}

	return nil
}
