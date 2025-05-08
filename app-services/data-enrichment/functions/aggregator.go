package functions

import (
	"strconv"
	"strings"
	"sync"

	"github.com/caio/go-tdigest/v4"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"hedge/app-services/data-enrichment/cache"
	"hedge/common/dto"
	"hedge/common/utils"
)

// Overall aggregation builder at across all devices and metrics
type AggregateBuilder struct {
	device2MetricAggregatorMap map[string]metricAggregator
	PauseDownSampling          bool
	nodeId                     string
	logger                     logger.LoggingClient
	cache                      cache.Manager
	mu                         sync.Mutex
}

type metricAggregator struct {
	mutex                  *sync.Mutex
	aggregateGroupByMetric map[string]*AggregateGroup
}

// Metrics computed for the same group ( eg deviceName) and same downsampling interval
type AggregateGroup struct {
	GroupId        string // by default, we do groupBy device instance
	BaseMetric     string
	ProfileName    string
	Tags           map[string]any
	ValueType      string
	StartTimeStamp int64
	LastTimeStamp  int64
	Digest         *tdigest.TDigest
}

func NewAggregateBuilder(
	lc logger.LoggingClient,
	cache cache.Manager,
	nodeId string,
) *AggregateBuilder {
	aggregateBuilder := new(AggregateBuilder)
	aggregateBuilder.logger = lc
	aggregateBuilder.nodeId = nodeId
	aggregateBuilder.cache = cache
	aggregateBuilder.device2MetricAggregatorMap = make(map[string]metricAggregator)
	return aggregateBuilder
}

// While collecting, we need BaseMetric, StartTime, ValueType Tag, ValueType, Digest
func NewAggregateGroup(reading dtos.BaseReading) *AggregateGroup {
	ag := AggregateGroup{
		GroupId:        reading.DeviceName,
		BaseMetric:     reading.ResourceName,
		ProfileName:    reading.ProfileName,
		Tags:           reading.Tags,
		ValueType:      reading.ValueType,
		StartTimeStamp: reading.Origin,
		LastTimeStamp:  reading.Origin,
	}
	ag.ValueType = reading.ValueType
	ag.Digest, _ = tdigest.New()
	return &ag
}

// Accumulates the metric data for aggregation and if done, returns the aggregate after draining the existing data
// In case of datatypes for which the aggregation doesn't make sense, just use downsampled value ( eg boolean).
// We might want to consider majority voting in this case. For now, take median value to keep it simple
func (ag *AggregateBuilder) BuildAggregates(event dtos.Event) (bool, *dto.Metrics) {
	ag.mu.Lock()
	deviceName := event.DeviceName
	if _, ok := ag.device2MetricAggregatorMap[deviceName]; !ok {
		// This needs under a lock so we don't create dups here,
		// the impact could be loosing a few metric points only initially, so not handling it
		ag.device2MetricAggregatorMap[deviceName] = metricAggregator{
			mutex:                  &sync.Mutex{},
			aggregateGroupByMetric: make(map[string]*AggregateGroup),
		}
	}
	aggregator := ag.device2MetricAggregatorMap[deviceName]
	ag.mu.Unlock()

	var metricDataGroups []dto.Data
	downSamplingConfig := ag.getDownSamplingConfig(event.ProfileName)
	doAggregateOrDownsampling := downSamplingConfig.DefaultDownsamplingIntervalSecs > 0
	var samplingInterval int64

	if doAggregateOrDownsampling {
		samplingInterval = downSamplingConfig.DefaultDownsamplingIntervalSecs * 1000000000
	}
	shouldSendRawData := ag.shouldSendRawData(event.Origin, doAggregateOrDownsampling)

	aggregator.mutex.Lock()
	defer aggregator.mutex.Unlock()

	if shouldSendRawData {
		metricDataGroups = make([]dto.Data, 0)
	}
	ag.logger.Debugf("shouldSendRawData: %v", shouldSendRawData)

	for _, reading := range event.Readings {
		metricName := reading.ResourceName

		isUnsupportedValueTypeForAggregation := reading.ValueType == common.ValueTypeBool ||
			reading.ValueType == common.ValueTypeString ||
			strings.HasSuffix(reading.ValueType, "Array")

		if shouldSendRawData || isUnsupportedValueTypeForAggregation {
			ag.logger.Debugf(
				"shouldSendRawData or Upsupported data type for aggregation for reading: %v",
				reading,
			)
			sample := buildMetricSampleFromReading(reading)
			metricDataGroups = append(metricDataGroups, sample)
		}

		if !doAggregateOrDownsampling || samplingInterval <= 0 ||
			isUnsupportedValueTypeForAggregation {
			continue
		}

		if _, ok := aggregator.aggregateGroupByMetric[metricName]; !ok {
			aggregator.aggregateGroupByMetric[metricName] = NewAggregateGroup(reading)
		}
		aggGroup := aggregator.aggregateGroupByMetric[metricName]

		// If the sampling interval is over, aggregate and drain the output to return

		if doAggregateOrDownsampling && aggGroup.StartTimeStamp+samplingInterval <= reading.Origin {

			aggGroup.LastTimeStamp = reading.Origin
			if metricDataGroups == nil {
				metricDataGroups = make([]dto.Data, 0)
			}
			// Compute aggregates
			aggrData := computeAggregatesAndBuildDataSamples(
				aggGroup,
				downSamplingConfig.Aggregates,
				shouldSendRawData,
			)
			for _, data := range aggrData {
				metricDataGroups = append(metricDataGroups, data)
			}
			digest, _ := tdigest.New()
			aggGroup.Digest = digest

		}
		// Only when the aggregation is complete in sampling interval do we initialize the StartTimeStamp
		if aggGroup.Digest.Count() == 0 {
			aggGroup.StartTimeStamp = reading.Origin // Reset the start time for aggregation
			aggGroup.LastTimeStamp = reading.Origin
		}
		addToAggregate(aggGroup, reading.ValueType, reading.Value)
	}

	if len(metricDataGroups) > 0 {
		metricGroup := dto.MetricGroup{
			Tags:    event.Tags,
			Samples: metricDataGroups,
		}

		if metricGroup.Tags == nil {
			ag.logger.Warnf("unexpected nil tag in metric event data: %v", event)
			metricGroup.Tags = make(map[string]any)
		}
		metricGroup.Tags["profileName"] = event.ProfileName
		metricGroup.Tags["deviceName"] = event.DeviceName
		aggregatesToPublish := dto.Metrics{
			IsCompressed: false,
			MetricGroup:  metricGroup,
		}
		return true, &aggregatesToPublish
	}
	return false, nil
}

func buildMetricSampleFromReading(reading dtos.BaseReading) dto.Data {

	metricData := dto.Data{
		Name:      reading.ResourceName,
		TimeStamp: reading.Origin,
		ValueType: reading.ValueType,
		Value:     reading.Value,
	}
	if reading.ValueType == common.ValueTypeBool {
		if reading.Value == "true" {
			metricData.Value = "1"
		} else {
			metricData.Value = "0"
		}
	}

	/*	if reading.ValueType != common.ValueTypeBool && reading.ValueType != common.ValueTypeString && !strings.HasSuffix(reading.ValueType, "Array") {
			valueFloat64, _ := parseSimpleValueToFloat64(reading.ValueType, reading.Value)
			metricData.ValueType = common.ValueTypeFloat64
			metricData.Value = valueFloat64
		} else {

		}*/
	return metricData

}

func addToAggregate(ag *AggregateGroup, valueType string, value string) {
	// Convert the datatype to float64 and addToAggregate to digest
	// Assume locked
	valueFloat64, err := utils.ParseSimpleValueToFloat64(valueType, value)
	if err == nil {
		err := ag.Digest.Add(valueFloat64)
		if err != nil {
			return
		}
	}
}

// Populates existing AggregateGroup->Metrics with the computed aggregate data
func computeAggregatesAndBuildDataSamples(
	ag *AggregateGroup,
	aggregateFunctionDefinitions []dto.Aggregate,
	pauseSampling bool,
) []dto.Data {

	// Assume already locked
	aggregateData := make([]dto.Data, 0)

	// Below only if downsampling is not paused, otherwise we get the same metric name twice
	if !pauseSampling {
		median := ag.Digest.Quantile(0.5)
		aggregateData = append(aggregateData, dto.Data{
			Name:      ag.BaseMetric,
			Value:     strconv.FormatFloat(median, 'E', -1, 64),
			TimeStamp: ag.LastTimeStamp,
			ValueType: common.ValueTypeFloat64,
		})
	}

	if aggregateFunctionDefinitions == nil {
		return aggregateData
	}
	for _, aggregateFuncDefn := range aggregateFunctionDefinitions {
		switch aggregateFuncDefn.FunctionName {
		case "avg", "mean":
			mean := ag.Digest.TrimmedMean(0, 1)
			aggregateData = append(
				aggregateData,
				buildAggregate(
					ag.BaseMetric,
					aggregateFuncDefn.FunctionName,
					mean,
					ag.LastTimeStamp,
				),
			)
		case "min":
			minimum := ag.Digest.Quantile(0)
			aggregateData = append(
				aggregateData,
				buildAggregate(ag.BaseMetric, "min", minimum, ag.LastTimeStamp),
			)
		case "max":
			maximum := ag.Digest.Quantile(1)
			aggregateData = append(
				aggregateData,
				buildAggregate(ag.BaseMetric, "max", maximum, ag.LastTimeStamp),
			)
		case "count":
			count := float64(ag.Digest.Count())
			aggregateData = append(
				aggregateData,
				buildAggregate(ag.BaseMetric, "count", count, ag.LastTimeStamp),
			)
		}
	}

	return aggregateData
}

func buildAggregate(
	baseMetric string,
	aggregateName string,
	aggregateValue float64,
	timeStamp int64,
) dto.Data {

	aggValue := dto.Data{
		Name:      baseMetric + "_" + aggregateName,
		Value:     strconv.FormatFloat(aggregateValue, 'E', -1, 64),
		TimeStamp: timeStamp,
		ValueType: common.ValueTypeFloat64,
	}
	return aggValue
}

// Returns whether to send raw data or not
func (ag *AggregateBuilder) shouldSendRawData(eventTimeStamp int64, doDownsampling bool) bool {
	if !doDownsampling {
		// In case there is no aggregation/downsampling, we always send raw data
		return true
	}

	conf, found := ag.cache.FetchRawDataConfiguration()
	if !found || conf == nil {
		return false
	}

	nodeRawDataConfig, _ := conf.(dto.NodeRawDataConfig)
	if !nodeRawDataConfig.SendRawData {
		return false
	}
	if nodeRawDataConfig.EndTime >= eventTimeStamp && nodeRawDataConfig.StartTime < eventTimeStamp {
		return true
	}
	return false
}

func (ag *AggregateBuilder) getDownSamplingConfig(profileName string) dto.DownsamplingConfig {
	conf, found := ag.cache.FetchDownSamplingConfiguration(profileName)
	if !found {
		return dto.DownsamplingConfig{
			DefaultDataCollectionIntervalSecs: 0,
			DefaultDownsamplingIntervalSecs:   0,
			Aggregates:                        nil,
		}
	}
	result, _ := conf.(dto.DownsamplingConfig)
	return result
}
