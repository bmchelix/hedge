package broker

import (
	"fmt"

	"hedge/common/dto"
	"hedge/common/utils"
	"hedge/edge-ml-service/pkg/dto/data"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

func (inf *MLBrokerInferencing) FilterMetricsByFeatureNames(
	edgexcontext interfaces.AppFunctionContext,
	eventData interface{},
) (bool, interface{}) {

	res := dto.MetricGroup{}

	//Bypass this function when run in the Hedge core
	metricsData, ok := eventData.(dto.MetricGroup)
	if !ok {
		edgexcontext.LoggingClient().Info("Skipping Metrics processing: FilterMetricsByFeatureName")
		return true, eventData
	}

	profileName, ok := metricsData.Tags["profileName"].(string)
	if !ok {
		return false, res
	}

	filteredData := make([]dto.Data, 0)
	for profile, features := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile {
		if profile != profileName {
			continue
		}

		for _, metric := range metricsData.Samples {
			for _, f := range features {
				if f.Name != metric.Name || f.Type != "METRIC" {
					continue
				}

				filteredData = append(filteredData, metric)
			}
		}
	}

	if len(filteredData) != 0 {
		return true, dto.MetricGroup{Samples: filteredData, Tags: metricsData.Tags}
	}
	return false, res
}

func (inf *MLBrokerInferencing) TakeMetricSample(
	edgexcontext interfaces.AppFunctionContext,
	eventData interface{},
) (bool, interface{}) {

	var res interface{}
	metricsData, ok := eventData.(dto.MetricGroup)
	if !ok {
		edgexcontext.LoggingClient().Info("Skipping Metrics processing: TakeMetricsSample")
		return true, eventData
	}

	profileName, ok := metricsData.Tags["profileName"].(string)
	if !ok {
		return false, res
	}

	accumulatedTSSamples := make([]*data.TSDataElement, len(metricsData.Samples))
	groupByMetaData := make(map[string]string, 0)
	groupByMetaData["deviceName"] = "GENERIC"

	var metaData = make(map[string]string)
	// Iterate over all Tags and fill-in against the groupBy metaData and/or metaData
	for metaKey, metaVal := range metricsData.Tags {
		if inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys != nil &&
			utils.Contains(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys, metaKey) {
			groupByMetaData[metaKey] = fmt.Sprintf("%v", metaVal)
		}
		metaData[metaKey] = fmt.Sprintf("%v", metaVal)
	}

	for i, sample := range metricsData.Samples {
		value, err := getValue(sample.Value, sample.ValueType)
		if err != nil {
			edgexcontext.LoggingClient().Warn(err.Error())
			continue
		}

		tsData := data.TSDataElement{
			Profile:         profileName,
			DeviceName:      "GENERIC",
			Timestamp:       sample.TimeStamp / 1000000,
			MetricName:      sample.Name,
			Value:           value,
			Tags:            metricsData.Tags,
			GroupByMetaData: groupByMetaData,
			MetaData:        metaData,
		}
		accumulatedTSSamples[i] = &tsData
	}
	return true, accumulatedTSSamples
}
