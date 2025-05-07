/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"

	"hedge/app-services/hedge-export/cmd/config"
	"hedge/common/dto"
	"hedge/common/models"
	"hedge/common/service"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/prometheus/prometheus/prompb"

	"time"

	"strconv"
	"strings"
)

type MetricExportService struct {
	ingestionTelemetryMonitor *IngestionTelemetryMonitor
	victoriaMetricsURL        string
	siteName                  string
	batchTimer                time.Duration
	MetricData                TSList
	batchSize                 int
	locker                    sync.Mutex
	persistOnError            bool
	vmHttpSender              service.HTTPSenderInterface
	bhomHttpSender            service.HTTPSenderInterface
}

type IngestionTelemetryMonitor struct {
	startTimeNanoSecs     int64
	bytesAdded            float64
	metricCount           int64
	elapsedTimeNanoSecs   float64
	ingestionRateKBPerSec float64
}

type TargetPayload struct {
	Type    string
	Payload interface{}
}

type ReturnObjs struct {
	TSPayload TargetPayload
}

type Label struct {
	Name  string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

type Datapoint struct {
	TimeMs int64
	Value  float64
}

type TimeSeries struct {
	Labels    []Label
	Datapoint Datapoint
}

type TSList []TimeSeries

const SAMPLING_INTERVAL_NANO_SECS = 1.0 * 60 * 1000000000

func NewMetricExportService(victoriaMetricsDbURL string, enableExportTelemetry bool, exportConfig config.AppExportConfig, persistOnError bool) *MetricExportService {
	metricService := new(MetricExportService)
	metricService.victoriaMetricsURL = victoriaMetricsDbURL
	metricService.batchSize = exportConfig.BatchSize
	metricService.batchTimer, _ = time.ParseDuration(exportConfig.BatchTimer)
	metricService.persistOnError = persistOnError
	cookie := service.NewDummyCookie()
	headers := make(map[string]string)

	metricService.vmHttpSender = service.NewHTTPSender(victoriaMetricsDbURL, "text/plain", persistOnError, headers, &cookie)

	// Get profile, Labels of all devices that has data collection enabled and keep them in cache
	if enableExportTelemetry {
		ingestionRateMonitor := new(IngestionTelemetryMonitor)
		ingestionRateMonitor.startTimeNanoSecs = time.Now().UnixNano()
		ingestionRateMonitor.elapsedTimeNanoSecs = 0
		ingestionRateMonitor.bytesAdded = 0
		metricService.ingestionTelemetryMonitor = ingestionRateMonitor
	} else {
		metricService.ingestionTelemetryMonitor = nil
	}
	metricService.MetricData = make(TSList, 0)

	return metricService
}

// TransformToTimeseries Output Format will either be Prometheus (for metrics to be sent to Victoria Metrics)
func (bm *MetricExportService) TransformToTimeseries(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {

	retObj := new(TargetPayload)
	if data == nil {
		retObj.Payload = errors.New("no Metric data received")
		return false, new(ReturnObjs)
	}

	lc := ctx.LoggingClient()
	lc.Debug("Transforming edgeX Metric to Output format")

	var promTags strings.Builder

	//data = bm.convertToMetrics(ctx, data)
	metricsArray := bm.CheckAndConvertToMetrics(ctx, data)
	if metricsArray == nil || len(metricsArray) == 0 {
		lc.Errorf("invalid metrics received, failed when casting to dto.Metrics")
		return false, nil
	}
	lc.Debugf("about to export metric %+v", metricsArray)
	metrics := metricsArray[0]
	var tsInMillis int64 = 0

	samples := metrics.MetricGroup.Samples
	tags := metrics.MetricGroup.Tags
	for i, sample := range samples {
		lc.Debugf("reading data:%d %s\n", i, sample)
		promVal := sample.Value
		tsInMillis = sample.TimeStamp / 1000000
		lc.Debugf("ts in milliseconds: %d\n", tsInMillis)
		if tsInMillis == 0 {
			tsInMillis = time.Now().UnixMilli()
		}

		promTags.WriteString(sample.Name) // Name of the metric
		promTags.WriteString(" {")
		begin := true

		// Need to populate Tags as part of data generation

		// Add all that is part of name-value pair in Labels in here
		for tagKey, tagValue := range tags {
			var tagStrValue string
			switch value := tagValue.(type) {
			case int:
				tagStrValue = strconv.Itoa(value)
			case int64:
				tagStrValue = fmt.Sprintf("%d", value)
			case float64:
				tagStrValue = strconv.FormatFloat(value, 'f', -1, 32)
			case float32:
				tagStrValue = fmt.Sprintf("%.0f", value)
			case string:
				tagStrValue = value
			default:
				switch tagValue {
				case nil:
					// ignore if tag is nil
					continue
				case reflect.ValueOf(tagValue).Kind() == reflect.Slice:
					// this is []interface{} etc
					arr, _ := tagValue.([]interface{})

					for i, val := range arr {
						// best attempt for now, we don't want to support such things
						if i == 0 {
							tagStrValue += fmt.Sprintf("%v", val)
						} else {
							tagStrValue += fmt.Sprintf(",%v", val)
						}
					}
				default:
					tagStrValue = fmt.Sprintf("%v", tagValue)
				}
			}

			if begin {
				promTags.WriteString(tagKey + "=\"" + tagStrValue + "\"")
				begin = false
			} else {
				promTags.WriteString(", " + tagKey + "=\"" + tagStrValue + "\"")
			}
		}

		promTags.WriteString("} ")
		promTags.WriteString(promVal)
		if tsInMillis > 0 {
			promTags.WriteString(" " + strconv.FormatInt(tsInMillis, 10))
		}
		promTags.WriteString("\n")
	}

	promStr := promTags.String()
	if promStr != "" {
		lc.Debugf("Prometheus TS Data: %s", promStr)
		if bm.ingestionTelemetryMonitor != nil {
			bm.updateIngestionRate(promStr, lc)
		}
	}
	return true, []byte(promStr)
}

func (bm *MetricExportService) ExportToTSDB(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {

	lc := ctx.LoggingClient()
	if data == nil {
		lc.Debugf("Metric data is nil")
		return true, nil
	}

	dataToSend := data.([]byte)
	if len(dataToSend) == 0 {
		lc.Debugf("Metric data is empty - %v", dataToSend)
		return true, nil
	}

	lc.Debug("Sending metrics to local Victoria")

	exportData := service.NewExportData(dataToSend, dataToSend)

	lc.Debugf("POSTing to local DB: URL=%s, Content-Type=%s, payload=%s", bm.victoriaMetricsURL, bm.vmHttpSender.GetMimeType(), string(data.([]byte)))
	ok, responseData := bm.vmHttpSender.HTTPPost(ctx, exportData)
	if !ok {
		errorMessage := fmt.Sprintf("Error saving metric data in pipeline %s", ctx.PipelineId())
		service.LogErrorResponseFromService(ctx.LoggingClient(), bm.victoriaMetricsURL, errorMessage, responseData)
		return false, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return true, nil
}

// CheckAndConvertToMetrics checks whether the data passed is predictions data and if so, converts it to Metrics
// Otherwise, the data is passed back as is.
func (bm *MetricExportService) CheckAndConvertToMetrics(ctx interfaces.AppFunctionContext, data interface{}) []dto.Metrics {

	lc := ctx.LoggingClient()
	lc.Tracef("data recieved from queue: %+v", data)

	// convert to bytes
	jsonBytes, err := util.CoerceType(data)
	if err != nil {
		lc.Errorf("Returning from CheckAndConvertToMetrics, error: %s", err.Error())
		return nil
	}

	// Try to obtain Metrics data
	var metrics dto.Metrics
	err = json.Unmarshal(jsonBytes, &metrics)
	if err == nil && metrics.MetricGroup.Samples != nil {
		lc.Debugf("Returning from CheckAndConvertToMetrics, metrics: %s", metrics)
		return []dto.Metrics{metrics}
	}

	// Try to obtain ML Prediction data and return nil if not successful
	var mlPredictions models.MLPrediction
	err = json.Unmarshal(jsonBytes, &mlPredictions)
	if err != nil {
		lc.Errorf("Returning from CheckAndConvertToMetrics, error getting MLPrediction data: %s", err.Error())
		return nil
	}

	lc.Tracef("data: %+v, mlPredictions: %+v", data, mlPredictions)

	var tags map[string]any
	if mlPredictions.Tags != nil {
		tags = mlPredictions.Tags
	} else {
		tags = make(map[string]any)
	}
	devices := ""
	for _, device := range mlPredictions.Devices {
		devices += device + ","
	}
	devices = strings.TrimRight(devices, ",")
	tags["deviceName"] = devices
	// Add other fields to the tag
	tags["entity_name"] = mlPredictions.EntityName
	tags["entity_type"] = mlPredictions.EntityType
	tags["algorithm_type"] = mlPredictions.MLAlgorithmType
	tags["algorithm_name"] = mlPredictions.MLAlgoName
	tags["ml_model_name"] = mlPredictions.ModelName

	var timeStamp int64
	if mlPredictions.Created == 0 {
		timeStamp = time.Now().UnixNano()
	} else {
		timeStamp = mlPredictions.Created * 1000000000
	}

	metricsArray := make([]dto.Metrics, 0)
	samples := make([]dto.Data, 0)
	valueType := ""
	if mlPredictions.MLAlgorithmType != helpers.TIMESERIES_ALGO_TYPE {
		value, valueType := bm.buildValue(mlPredictions, valueType, lc)
		if value == "" {
			return nil
		}
		metricData := dto.Data{Name: mlPredictions.PredictionName, TimeStamp: timeStamp, Value: value, ValueType: valueType}
		samples = append(samples, metricData)
		metrics = dto.Metrics{
			IsCompressed: false,
			MetricGroup: dto.MetricGroup{
				Tags: tags, Samples: samples,
			},
		}
		metricsArray = append(metricsArray, metrics)
		return metricsArray
	}
	// build multiple samples for each prediction TS output
	//mlPredictions.Prediction
	// Build samples for TS

	predictionMap, ok := mlPredictions.Prediction.(map[string]interface{})
	if !ok {
		return nil
	}

	// this level it is predictions, min and max boundary
	for name, predArray := range predictionMap {
		// The array is the prediction over different timestamps
		predictions, ok := predArray.([]interface{})
		if !ok {
			lc.Warnf("mismatched data type: expected array, got: %v", predArray)
			continue
		}

		for _, predInterface := range predictions {
			prediction, ok := predInterface.(map[string]interface{})
			if !ok {
				continue
			}

			var futureTimestamp int64
			if _, ok = prediction["Timestamp"].(float64); ok {
				futureTimestamp = int64(prediction["Timestamp"].(float64))
			}
			for predName, predVal := range prediction {
				if predName != "Timestamp" {
					value := fmt.Sprintf("%v", predVal)
					profileNMetric := strings.Split(predName, "#")
					forcastMetricName := fmt.Sprintf("%s_%s", profileNMetric[1], name)
					//data := dto.Data
					// every predVal is for a different futureTime that we need to capture in futureTimestamp field as tags
					// It is assumed that this is handled during plotting in grafana or via a datasource plugin to swap timestamp value during display, but query part can be just the time when the forcast was done
					samples = []dto.Data{
						{
							Name:      forcastMetricName,
							TimeStamp: mlPredictions.Created,
							Value:     value,
							ValueType: "Float64",
						},
					}
					futureTimestampTag := map[string]interface{}{
						"futureTimestamp": futureTimestamp,
					}
					tagWithFutureTime := make(map[string]any)
					for k, v := range tags {
						tagWithFutureTime[k] = v
					}
					tagWithFutureTime["futureTimestamp"] = futureTimestampTag
					metrics = dto.Metrics{
						IsCompressed: false,
						MetricGroup: dto.MetricGroup{
							Tags: tagWithFutureTime, Samples: samples,
						},
					}
					metricsArray = append(metricsArray, metrics)
				}
			}
		}
	}

	lc.Tracef("metricArray: %+v", metricsArray)
	return metricsArray
}

func (bm *MetricExportService) buildValue(mlPredictions models.MLPrediction, valueType string, lc logger.LoggingClient) (string, string) {
	value := ""
	predictionData := mlPredictions.Prediction
	switch t := predictionData.(type) {
	case float64:
		valueType = "Float64"
		value = fmt.Sprintf("%v", t)
	case float32:
		valueType = "Float32"
		value = fmt.Sprintf("%v", t)
	case int:
		valueType = "Int"
		value = fmt.Sprintf("%v", t)
	default:
		lc.Warnf("unsupported data type for prediction value: %+v", predictionData)
	}
	return value, valueType
}

func (bm *MetricExportService) ProcessConfigUpdates(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*config.AppExportConfig)
	if !ok {
		return
	}
	tempTimer, _ := time.ParseDuration(updated.BatchTimer)

	if bm.batchSize != updated.BatchSize {
		bm.batchSize = updated.BatchSize
	}
	if bm.batchTimer != tempTimer {
		bm.batchTimer = tempTimer
	}
	if bm.persistOnError != updated.PersistOnError {
		bm.persistOnError = updated.PersistOnError
		cookie := service.NewDummyCookie()
		headers := make(map[string]string)
		bm.vmHttpSender = service.NewHTTPSender(bm.victoriaMetricsURL, "text/plain", bm.persistOnError, headers, &cookie)
	}
}

func (bm *MetricExportService) updateIngestionRate(data string, lc logger.LoggingClient) {
	bm.locker.Lock()
	ir := bm.ingestionTelemetryMonitor
	defer bm.locker.Unlock()

	ir.bytesAdded += float64(len(data))
	// No of lines in data = no of metrics
	ir.metricCount += int64(strings.Count(data, "\n"))
	now := time.Now().UnixNano()
	ir.elapsedTimeNanoSecs = float64(now - ir.startTimeNanoSecs)
	//lc.Infof("metricCount: %d, bytes added total: %f, elapsedTimeNanoSecs: %f\n",ir.metricCount, ir.bytesAdded, ir.elapsedTimeNanoSecs)
	if ir.elapsedTimeNanoSecs > SAMPLING_INTERVAL_NANO_SECS {
		ir.ingestionRateKBPerSec = ir.bytesAdded * 1000000 / ir.elapsedTimeNanoSecs
		lc.Debugf("#Metrics ingested: %d, Data Ingestion Rate: %f KB/sec\n", ir.metricCount, ir.ingestionRateKBPerSec)
		// Now reset the data
		ir.bytesAdded = 0.0
		ir.metricCount = 0
		ir.startTimeNanoSecs = now
		ir.elapsedTimeNanoSecs = 0
	}
}

// Unused method for CA
/*
func (bm *MetricExportService) sendByTimer(ctx interfaces.AppFunctionContext) {
	lc := ctx.LoggingClient()
	lc.Debug("Starting new timer")
	exportTimer = time.NewTimer(bm.batchTimer)
	<-exportTimer.C
	lc.Debug("Metric sent by timer")
	var protobytes []byte
	protobytes = nil
	var err error
	if len(bm.MetricData) > 0 {
		promWR := bm.toPromWriteRequest(bm.MetricData)
		if len(promWR.Timeseries) > 0 {
			protobytes, err = proto.Marshal(promWR)
			if err != nil {
				lc.Errorf("Failed to marshal data in prometheus format")
				protobytes = nil
			}
		}
		bm.ExportToBHOM(ctx, protobytes)
	}
}
*/
// toPromWriteRequest converts a list of TimeSeries to a Prometheus proto write request.
func (bm *MetricExportService) toPromWriteRequest(lc logger.LoggingClient) *prompb.WriteRequest {
	if len(bm.MetricData) == 0 {
		return &prompb.WriteRequest{}
	}
	metricDataCount := len(bm.MetricData)
	lc.Debugf("Length of bm.MetricData: %d", metricDataCount)
	promTS := make([]prompb.TimeSeries, metricDataCount)
	lc.Debugf("Length of promTS: %d", len(promTS))
	for i, ts := range bm.MetricData {
		lc.Debugf("ts value: %v", ts)
		if ts.Labels == nil {
			lc.Errorf("metric data Labels is nil for,full batch being dropped, ts : %v", ts)
			return nil
		}
		nLabels := len(ts.Labels)
		if nLabels < 1 || nLabels > 64 {
			lc.Errorf("metric data Label size is out of bounds, full batch being dropped, ts : %v", ts)
			return nil
		}

		labels := make([]prompb.Label, len(ts.Labels))
		for j, label := range ts.Labels {
			labels[j] = prompb.Label{Name: label.Name, Value: label.Value}
		}

		sample := []prompb.Sample{{
			// Timestamp is int milliseconds for remote write.
			//Timestamp: ts.Datapoint.TimeMs.UnixNano() / int64(time.Millisecond),
			Timestamp: ts.Datapoint.TimeMs,
			Value:     ts.Datapoint.Value,
		}}
		if i < metricDataCount {
			promTS[i] = prompb.TimeSeries{Labels: labels, Samples: sample}
		} else {
			lc.Errorf("MetricData Count more than len(metricData), i=%d, full data: %v", i, bm.MetricData)
		}

	}

	return &prompb.WriteRequest{
		Timeseries: promTS,
	}
}

func getDataValueAsStr(valueType string, value interface{}, lc logger.LoggingClient) (bool, string) {

	if valueType == "" || strings.Contains(valueType, "Array") || value == nil {
		// Unsupported type
		lc.Warnf("Unsupported valueType or empty value: %s, value: %v\n", valueType)
		return false, ""
	}
	var promVal = ""
	isSuccess := true
	var val interface{}

	if valueType == common.ValueTypeFloat64 {
		val, isSuccess = value.(float64)
		if isSuccess {
			promVal = fmt.Sprintf("%f", val)
		}
	} else if valueType == common.ValueTypeFloat32 {
		val, isSuccess = value.(float32)
		if isSuccess {
			promVal = fmt.Sprintf("%f", val)
		}
	} else if valueType == common.ValueTypeBool {
		val, isSuccess = value.(bool)
		if isSuccess {
			if val.(bool) {
				promVal = "1"
			} else {
				promVal = "0"
			}
		}

	} else if valueType == common.ValueTypeUint64 {
		val, isSuccess = value.(uint64)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeUint32 {
		val, isSuccess = value.(uint32)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeUint16 {
		val, isSuccess = value.(uint16)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeUint8 {
		val, isSuccess = value.(uint8)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeInt64 {
		val, isSuccess = value.(int64)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeInt32 {
		val, isSuccess = value.(int32)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeInt16 {
		val, isSuccess = value.(int16)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else if valueType == common.ValueTypeInt8 {
		val, isSuccess = value.(int8)
		if isSuccess {
			promVal = fmt.Sprintf("%d", val)
		}
	} else {
		lc.Errorf("Unsupported valueType: %s, value=%v", valueType, value)
		isSuccess = false
	}

	return isSuccess, promVal
}

func valueIntToFloat64(lc logger.LoggingClient, valueType string, value interface{}) (bool, float64) {

	if value == "" || strings.Contains(valueType, "Array") || strings.Contains(valueType, common.ValueTypeString) {
		// Unsupported type
		lc.Warnf("Unsupported dataType or empty value: %s", valueType)
		return false, 0
	}
	var floatValue float64 = 0
	isSuccess := true

	if valueType == common.ValueTypeFloat64 {
		return true, value.(float64)

	} else if valueType == common.ValueTypeFloat32 {
		floatValue = float64(value.(float32))
	} else if valueType == common.ValueTypeBool {
		b := value.(bool)
		if b {
			floatValue = float64(1)
		} else {
			floatValue = float64(0)
		}

	} else if valueType == common.ValueTypeUint64 {
		floatValue = float64(value.(uint64))
	} else if valueType == common.ValueTypeUint32 {
		floatValue = float64(value.(uint32))
	} else if valueType == common.ValueTypeUint16 {
		floatValue = float64(value.(uint16))
	} else if valueType == common.ValueTypeUint8 {
		floatValue = float64(value.(uint8))
	} else if valueType == common.ValueTypeInt64 {
		floatValue = float64(value.(int64))
	} else if valueType == common.ValueTypeInt32 {
		floatValue = float64(value.(int32))
	} else if valueType == common.ValueTypeInt16 {
		floatValue = float64(value.(int16))
	} else if valueType == common.ValueTypeInt8 {
		floatValue = float64(value.(int8))
	} else {
		lc.Errorf("Unsupported type=%s, value=%v", valueType, value)
		isSuccess = false
	}

	return isSuccess, floatValue
}

func getValue(reading dtos.BaseReading, lc logger.LoggingClient) string {
	if reading.SimpleReading.Value != "" {
		return reading.SimpleReading.Value
	}
	lc.Errorf("BinaryReading or ObjectReading reading type not supported")
	return ""
}

// Unused method for CA
/*func (bm *MetricExportService) sendByTimer(ctx interfaces.AppFunctionContext) {
	lc := ctx.LoggingClient()
	lc.Debug("Starting new timer")
	exportTimer = time.NewTimer(bm.batchTimer)
	<-exportTimer.C
	lc.Debug("Metric sent by timer")
	var protobytes []byte
	protobytes = nil
	lc.Debug("Metric sent by batch")
	var err error
	promWR := batchArr.toPromWriteRequest()
	if len(promWR.Timeseries) > 0 {
		protobytes, err = proto.Marshal(promWR)
		if err != nil {
			lc.Errorf("Failed to marshal data in prometheus format")
			protobytes = nil
		}
	}
	bm.ExportToBHOM(ctx, protobytes)
}*/

func addLabels(tags map[string]any) []Label {
	labels := make([]Label, 0)
	for tagKey, tagValue := range tags {
		var tagStrValue string
		switch tagValue.(type) {
		case int:
			tagStrValue = strconv.Itoa(tagValue.(int))
		case int64:
			tagStrValue = fmt.Sprintf("%d", tagValue.(int64))
		case float64:
			tagStrValue = strconv.FormatFloat(tagValue.(float64), 'f', -1, 32)
		case float32:
			tagStrValue = fmt.Sprintf("%.0f", tagValue.(float32))
		case string:
			tagStrValue = tagValue.(string)
		default:
			if reflect.ValueOf(tagValue).Kind() == reflect.Slice {
				// this is []interface{} etc
				arr := tagValue.([]interface{})
				for i, val := range arr {
					// best attempt for now, we don't want to support such things
					if i == 0 {
						tagStrValue += fmt.Sprintf("%v", val)
					} else {
						tagStrValue += fmt.Sprintf(",%v", val)
					}
				}
			} else {
				tagStrValue = fmt.Sprintf("%v", tagValue)
			}
		}

		if tagStrValue != "" {
			lb := Label{Name: tagKey, Value: tagStrValue}
			labels = append(labels, lb)
		}
	}
	return labels
}
