/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training_data_test

import (
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	mlagent "hedge/edge-ml-service/pkg/training-data"
	svcmocks "hedge/mocks/hedge/common/service"
	helpersmocks "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
	"bytes"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

var mockResponse *http.Response

func createMockResponse() {
	mockTimeSeriesResponse := data.TimeSeriesResponse{
		Status: "success",
		Data: data.TimeSeriesData{
			ResultType: "matrix",
			Result: []data.MetricResult{
				{
					Metric: map[string]interface{}{
						"__name__":    "TestMetric",
						"deviceName":  "TestDevice",
						"profileName": "TestProfile",
					},
					Values: []interface{}{
						[]interface{}{float64(1604289920), "123.45"},
					},
				},
			},
		},
	}

	mockBody, _ := json.Marshal(mockTimeSeriesResponse)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}
}

func Test_Execute_Passed(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{
		0: {123.45, 1.0},
		1: {678.90, 2.0},
	}

	mockedMlStorage.On("GetMLModelConfigFileName", true).Return(TRAINING_DATA_FILE)
	mockedMlStorage.On("GetTrainingDataFileName").Return(TRAINING_DATA_FILE)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)
	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFile", mock.Anything, mock.Anything).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.Execute()
	assert.NoError(t, err)
}

func Test_Execute_Failed_ErrorInGenerateConfig(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	createMockResponse()

	mockedMlStorage.On("GetTrainingDataFileName").Return(TRAINING_DATA_FILE)
	mockedMlStorage.On("GetMLModelConfigFileName", true).Return(TRAINING_DATA_FILE)
	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(errors.New("error init file"))

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
	}

	err := dc.Execute()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error init file")

}

func Test_ExecuteWithFileNames_Passed(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}

	dataTransformToFeature := map[int64][]interface{}{
		0: {123.45, 1.0},
		1: {678.90, 2.0},
		2: {678.90, 2.0},
	}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.NoError(t, err)

}

func Test_ExecuteWithFileNames_Failed_ErrorInInitFile(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{
		0: {123.45, 1.0},
		1: {678.90, 2.0},
		2: {678.90, 2.0},
	}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(errors.New("error init file"))
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.Error(t, err)
	assert.EqualError(t, err, "error init file")

}

func Test_ExecuteWithFileNames_Failed_ErrorInGetTrainingDataSample(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("error fetching data from data provider"))
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.EqualError(t, err, "error fetching data from data provider")

}

func Test_ExecuteWithFileNames_Failed_ErrorInAddTrainingDataToFileWithFileName(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{
		0: {123.45, 1.0},
		1: {678.90, 2.0},
		2: {678.90, 2.0},
	}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(errors.New("error creating file : dummyFile, error: no space left/n"))
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.EqualError(t, err, "error creating file : dummyFile, error: no space left/n")

}

func Test_ExecuteWithFileNames_Failed_CountZero(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.EqualError(t, err, "item not found")

}

func Test_ExecuteWithFileNames_Failed_ErrorInAnalyzeRowCounts(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{
		0: {123.45, 1.0},
	}

	mockedMlStorage.On("InitializeFile", TRAINING_DATA_FILE).Return(nil)
	mockedMlStorage.On("AddTrainingDataToFileWithFileName", mock.Anything, mock.Anything, TRAINING_DATA_FILE).Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.ExecuteWithFileNames(TRAINING_DATA_FILE)
	assert.EqualError(t, err, "Not enough data has been collected for training the ML model: testFile.csv, no group names have >= 100 records")

}

func Test_GenerateSample_Passed(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289922,
				MetricName:      "TestMetric3",
				Value:           101.11,
				MetaData:        map[string]string{"key3": "value3"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{0: {123.45, 1.0}}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedMlStorage.On("GetTrainingDataFileName").Return(TRAINING_DATA_FILE)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	wantedResults := []string{
		"featureKey1,featureKey2",
		"123.45,1",
	}

	returnedResults, err := dc.GenerateSample(1)

	assert.NoError(t, err)

	if !reflect.DeepEqual(returnedResults, wantedResults) {
		t.Errorf("GenerateSample() got = %v, want %v", returnedResults, wantedResults)
	}

}

func Test_GenerateSample_Failed_ErrorInTrainingData(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289922,
				MetricName:      "TestMetric3",
				Value:           101.11,
				MetaData:        map[string]string{"key3": "value3"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{0: {123.45, 1.0}}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedMlStorage.On("GetTrainingDataFileName").Return(TRAINING_DATA_FILE)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)
	mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("error fetching data from data provider"))

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	wantedResults := []string{""}

	returnedResults, err := dc.GenerateSample(0)

	assert.Error(t, err)
	assert.EqualError(t, err, "error fetching data from data provider")
	if !reflect.DeepEqual(wantedResults, returnedResults) {
		t.Errorf("GenerateSample() got = %v, want %v", wantedResults, returnedResults)
	}

}

func Test_GenerateSample_Failed_CountZero(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289921,
				MetricName:      "TestMetric2",
				Value:           678.90,
				MetaData:        map[string]string{"key2": "value2"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289922,
				MetricName:      "TestMetric3",
				Value:           101.11,
				MetaData:        map[string]string{"key3": "value3"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	dataTransformToFeature := map[int64][]interface{}{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedMlStorage.On("GetTrainingDataFileName").Return(TRAINING_DATA_FILE)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	returnedResults, err := dc.GenerateSample(0)

	assert.EqualError(t, err, "item not found")
	assert.Equal(t, []string{"featureKey1,featureKey2"}, returnedResults)

}

func Test_AnalyzeRowCounts_Passed_SufficientRows(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mlModelConfigName := "TestMLModel"
	rowCountByGroup := &sync.Map{}
	rowCountByGroup.Store("Group1", 50)
	rowCountByGroup.Store("Group2", 150)

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.AnalyzeRowCounts(rowCountByGroup, mlModelConfigName)

	assert.NoError(t, err, "Expected no error when at least one group has sufficient rows")

}

func Test_AnalyzeRowCounts_Failed_InsufficientRows(t *testing.T) {
	originalRowCountLimit := mlagent.RowCountLimit
	mlagent.RowCountLimit = 2
	t.Cleanup(func() {
		mlagent.RowCountLimit = originalRowCountLimit
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mlModelConfigName := "TestMLModel"

	rowCountByGroup := &sync.Map{}
	rowCountByGroup.Store("Group1", 1)
	rowCountByGroup.Store("Group2", 1)
	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.AnalyzeRowCounts(rowCountByGroup, mlModelConfigName)

	assert.Error(t, err, "Expected an error when no groups have sufficient rows")
	assert.EqualError(t, err, "Not enough data has been collected for training the ML model: TestMLModel, no group names have >= 100 records")

}

func Test_GenerateConfig_Passed(t *testing.T) {
	trainingConfigFile := "mockTrainingConfig.json"
	t.Cleanup(func() {
		_ = os.Remove(trainingConfigFile)
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedMlStorage.On("InitializeFile", "mockTrainingConfig.json").Return(nil)
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.GenerateConfig(trainingConfigFile)

	assert.NoError(t, err)
	mockedMlStorage.AssertCalled(t, "InitializeFile", trainingConfigFile)
	mockedPreProcessor.AssertCalled(t, "GetMLModelConfig")
	mockedPreProcessor.AssertCalled(t, "GetMLAlgorithmDefinition")

}

func Test_GenerateConfig_Failed_ErrorInInitializeFile(t *testing.T) {
	trainingConfigFile := "mockTrainingConfig.json"
	t.Cleanup(func() {
		_ = os.Remove(trainingConfigFile)
		_ = os.Remove(TRAINING_DATA_FILE)
	})

	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedMlStorage.On("InitializeFile", "mockTrainingConfig.json").Return(errors.New("file initialization error"))

	dc := &mlagent.DataCollector{
		PreProcessor:         &mockedPreProcessor,
		LoggingClient:        u.AppService.LoggingClient(),
		AppConfig:            appConfig,
		DataStoreProvider:    mockedDataSourceProvider,
		BaseQuery:            "{ __name__=~\"\"}",
		ChunkSize:            5,
		MlStorage:            &mockedMlStorage,
		TrainingDataFileName: TRAINING_DATA_FILE,
		Client:               &mockHttpClient,
	}

	err := dc.GenerateConfig(trainingConfigFile)

	assert.Error(t, err)
	assert.EqualError(t, err, "file initialization error")
	mockedMlStorage.AssertCalled(t, "InitializeFile", trainingConfigFile)

}

func Test_GetMlStorage(t *testing.T) {
	dc := &mlagent.DataCollector{
		MlStorage: &helpersmocks.MockMLStorageInterface{},
	}
	result := dc.GetMlStorage()
	if result != dc.MlStorage {
		t.Errorf("GetMlStorage() = %v, want %v", result, dc.MlStorage)
	}
}

func Test_GetSampleDataSetFromDB_Passed(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	wantedResults := [][]*data.TSDataElement{
		{
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{},
				GroupByMetaData: map[string]string{},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}
	returnedResults, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.NoError(t, err)
	if !reflect.DeepEqual(returnedResults, wantedResults) {
		t.Errorf("GetSampleDataSetFromDB() got = %v, want %v", returnedResults, wantedResults)
	}
}

func Test_GetSampleDataSetFromDB_Failed_HTTPClientError(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("error fetching data from data provider"))

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	_, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.Error(t, err)
	assert.EqualError(t, err, "error fetching data from data provider")
}

func Test_GetSampleDataSetFromDB_Failed_HTTPStatusNot200(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)

	mockResponse = &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
	}
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	_, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch data, http status")
}

func Test_GetSampleDataSetFromDB_Failed_JSONDecodingError(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)

	mockResponse = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("invalid json")),
	}
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	_, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func Test_GetSampleDataSetFromDB_Failed_MissingBaseQuery(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	_, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.Error(t, err)
	assert.EqualError(t, err, "Error with Training data, training job will be skipped")
}

func Test_GetSampleDataSetFromDB_Failed_NewRequestFails(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("http://%41:8080/")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	_, err := dc.GetSampleDataSetFromDB(startTime, endTime)

	assert.Error(t, err)
	assert.EqualError(t, err, "parse \"http://%41:8080//query_range\": invalid URL escape \"%41\"")
}

func Test_GetSampleDataSetFromDB_Passed_DeviceNameUnknown(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockTimeSeriesResponse := data.TimeSeriesResponse{
		Status: "success",
		Data: data.TimeSeriesData{
			ResultType: "matrix",
			Result: []data.MetricResult{
				{
					Metric: map[string]interface{}{
						"__name__":    "TestMetric",
						"profileName": "TestProfile",
					},
					Values: []interface{}{
						[]interface{}{float64(1604289920), "123.45"},
					},
				},
			},
		},
	}

	mockBody, _ := json.Marshal(mockTimeSeriesResponse)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	results, err := dc.GetSampleDataSetFromDB(startTime, endTime)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "Unknown", results[0][0].DeviceName)
}

func Test_GetSampleDataSetFromDB_Passed_FeatureKeyFound(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()
	featuresToIndex := map[string]int{"TestProfile#__name__": 0}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(featuresToIndex)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	results, err := dc.GetSampleDataSetFromDB(startTime, endTime)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Contains(t, results[0][0].MetaData, "__name__")
	assert.Equal(t, "TestMetric", results[0][0].MetaData["__name__"])
}

func Test_GetSampleDataSetFromDB_Passed_ContainsGroupOrJoinKeys(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	mockTimeSeriesResponse := data.TimeSeriesResponse{
		Status: "success",
		Data: data.TimeSeriesData{
			ResultType: "matrix",
			Result: []data.MetricResult{
				{
					Metric: map[string]interface{}{
						"__name__":    "TestMetric",
						"profileName": "TestProfile",
						"device":      "TestDevice",
					},
					Values: []interface{}{
						[]interface{}{float64(1604289920), "7.89"},
					},
				},
			},
		},
	}

	mockBody, _ := json.Marshal(mockTimeSeriesResponse)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}
	featuresToIndex := map[string]int{"TestProfile#__name__": 0}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockmlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName", "TestProfile"}
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(featuresToIndex)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	results, err := dc.GetSampleDataSetFromDB(startTime, endTime)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "TestDevice", results[0][0].GroupByMetaData["deviceName"])
}

func Test_GetSampleDataSetFromDB_Passed_FailToConvertFloatValue(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockedMlStorage := helpersmocks.MockMLStorageInterface{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	mockTimeSeriesResponse := data.TimeSeriesResponse{
		Status: "success",
		Data: data.TimeSeriesData{
			ResultType: "matrix",
			Result: []data.MetricResult{
				{
					Metric: map[string]interface{}{
						"__name__":    "TestMetric",
						"profileName": "TestProfile",
						"device":      "TestDevice",
					},
					Values: []interface{}{
						[]interface{}{1604289920, "invalid_float"},
					},
				},
			},
		},
	}

	mockBody, _ := json.Marshal(mockTimeSeriesResponse)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	mockedDataSourceProvider.On("GetDataURL").Return("http://webspaceconfig.de")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		MlStorage:         &mockedMlStorage,
		Client:            &mockHttpClient,
	}
	returnedResults, err := dc.GetSampleDataSetFromDB(startTime, endTime)
	assert.NoError(t, err)
	assert.Equal(t, [][]*data.TSDataElement{}, returnedResults)
}

func Test_GetTrainingDataSample_Passed(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()
	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}

	mockedDataSourceProvider.On("GetDataURL").Return("https://mock.datastore.com")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	dataForTransformToFeature := map[int64][]interface{}{0: {123.45, 1.0}}
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataForTransformToFeature, true)

	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320
	rowCountByGroup := &sync.Map{}
	wantedResult1 := [][]string{
		{"123.45", "1"},
	}

	wantedResult2 := []string{"featureKey1", "featureKey2"}

	returnedResults1, returnedResults2, err := dc.GetTrainingDataSample(startTime, endTime, rowCountByGroup)
	assert.NoError(t, err)
	if !reflect.DeepEqual(returnedResults1, wantedResult1) {
		t.Errorf("GetTrainingDataSample() got = %v, want %v", returnedResults1, wantedResult1)
	}
	if !reflect.DeepEqual(returnedResults2, wantedResult2) {
		t.Errorf("GetTrainingDataSample() got1 = %v, want %v", returnedResults2, wantedResult2)
	}
}

func Test_GetTrainingDataSample_Failed_ErrorInGetSampleDataSetFromDB(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}

	mockedDataSourceProvider.On("GetDataURL").Return("https://mock.datastore.com")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1})
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	dataTransformToFeature := map[int64][]interface{}{0: {123.45, 1.0}}
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataTransformToFeature, true)
	mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("error fetching data from data provider"))

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320
	rowCountByGroup := &sync.Map{}
	_, _, err := dc.GetTrainingDataSample(startTime, endTime, rowCountByGroup)
	assert.Error(t, err)
	assert.EqualError(t, err, "error fetching data from data provider")
}

func Test_GetTrainingDataSample_Failed_FeatureCountMismatch(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	mockedDataSourceProvider.On("GetDataURL").Return("https://mock.datastore.com")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1}) // Expecting 2 columns
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}
	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)

	dataForTransformToFeature := map[int64][]interface{}{
		0: {123.45},
	}
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(dataForTransformToFeature, true)

	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320
	rowCountByGroup := &sync.Map{}

	_, _, err := dc.GetTrainingDataSample(startTime, endTime, rowCountByGroup)

	assert.Error(t, err)
	assert.EqualError(t, err, "issue with training data, expected feature count: 2, found: 1\n")
}

func Test_GetTrainingDataSample_FeaturesIncomplete(t *testing.T) {
	mockedPreProcessor := helpersmocks.MockPreProcessorInterface{}
	mockedDataSourceProvider := &svcmocks.MockDataStoreProvider{}
	mockHttpClient := svcmocks.MockHTTPClient{}
	createMockResponse()

	mockedDataSourceProvider.On("GetDataURL").Return("https://mock.datastore.com")
	mockedDataSourceProvider.On("SetAuthHeader", mock.Anything).Return()
	mockedPreProcessor.On("GetFeaturesToIndex").Return(map[string]int{"featureKey1": 0, "featureKey2": 1}) // Expecting 2 features
	mockedPreProcessor.On("GetMLModelConfig").Return(mockmlModelConfig)
	groupSamples := map[string][]*data.TSDataElement{
		"group1": {
			&data.TSDataElement{
				Profile:         "TestProfile",
				Timestamp:       1604289920,
				MetricName:      "TestMetric",
				Value:           123.45,
				MetaData:        map[string]string{"key1": "value1"},
				GroupByMetaData: map[string]string{"groupKey": "group1"},
				DeviceName:      "TestDevice",
				Tags:            nil,
			},
		},
	}

	mockedPreProcessor.On("GroupSamples", mock.Anything).Return(groupSamples)
	mockedPreProcessor.On("TransformToFeature", mock.AnythingOfType("string"), mock.AnythingOfType("[]*data.TSDataElement")).
		Return(nil, false)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)

	dc := &mlagent.DataCollector{
		PreProcessor:      &mockedPreProcessor,
		LoggingClient:     u.AppService.LoggingClient(),
		AppConfig:         appConfig,
		DataStoreProvider: mockedDataSourceProvider,
		BaseQuery:         "{ __name__=~\"\"}",
		ChunkSize:         5,
		Client:            &mockHttpClient,
	}

	var startTime, endTime int64
	startTime = 1604289920
	endTime = 1604304320
	rowCountByGroup := &sync.Map{}

	returnedResults1, returnedResults2, err := dc.GetTrainingDataSample(startTime, endTime, rowCountByGroup)

	assert.Empty(t, returnedResults1)
	assert.Equal(t, []string{"featureKey1", "featureKey2"}, returnedResults2)
	assert.NoError(t, err)

}

func Test_SetPreProcessor(t *testing.T) {
	dc := &mlagent.DataCollector{
		PreProcessor: nil,
	}

	newPreProcessor := &helpersmocks.MockPreProcessorInterface{}
	dc.SetPreProcessor(newPreProcessor)
	if dc.PreProcessor != newPreProcessor {
		t.Errorf("SetPreProcessor() = %v, want %v", dc.PreProcessor, newPreProcessor)
	}
}

func Test_GetPreProcessor(t *testing.T) {
	dc := &mlagent.DataCollector{
		PreProcessor: &helpersmocks.MockPreProcessorInterface{},
	}
	result := dc.GetPreProcessor()
	if result != dc.PreProcessor {
		t.Errorf("GetPreProcessor() = %v, want %v", result, dc.PreProcessor)
	}
}

func Test_BuildQuery(t *testing.T) {

	featuresByProfile := map[string][]config.Feature{
		"profile1": {
			{Name: "feature1", Type: "METRIC"},
			{Name: "feature2", Type: "METRIC"},
		},
		"profile2": {
			{Name: "feature3", Type: "METRIC"},
			{Name: "feature4", Type: "METRIC"},
		},
	}

	trainingConfig := &config.MLModelConfig{}
	trainingConfig.MLDataSourceConfig.FeaturesByProfile = featuresByProfile
	trainingConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters = []config.FilterDefinition{
		{Label: "Label1", Operator: "EQUALS", Value: "Value1"},
		{Label: "Label2", Operator: "CONTAINS", Value: "Value2"},
		{Label: "Label2", Operator: "EXCLUDES", Value: "Value3"},
	}
	result := mlagent.BuildQuery(trainingConfig)

	// Assert
	expectedQueryPattern := regexp.MustCompile(`{ __name__=~"feature1|feature2|feature3|feature4", Label1 =~".*Value1.*", Label2 =~".*Value2.*", Label2 !="Value3"}`)
	assert.True(t, expectedQueryPattern.MatchString(result), "Unexpected result for BuildQuery")
}

func Test_fileExists_FileFound_Passed(t *testing.T) {
	existingFile := "testfile.txt"
	file, _ := os.Create(existingFile)
	defer file.Close()

	t.Cleanup(func() {
		if err := os.Remove(existingFile); err != nil {
			t.Errorf("Failed to delete file: %v", err)
		}
	})

	result := mlagent.FileExists(existingFile)
	if !result {
		t.Errorf(" FileExists returned false, but expected true")
	}
}

func Test_fileExists_FileNotFound_Passed(t *testing.T) {
	nonExistingFile := "nonexistentfile.txt"
	result := mlagent.FileExists(nonExistingFile)
	if result {
		t.Errorf(" FileExists returned true, but expected false")
	}
}

func TestSetFetchStatusManager(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "status.json")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}

	dc := &mlagent.DataCollector{}
	dc.SetFetchStatusManager(tmpFile.Name())

	if dc.FetchStatusManager == nil {
		t.Error("FetchStatusManager is nil, expected a valid manager.")
	}
	_ = os.Remove("status.json")
}

func TestGetFetchStatusManager(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "status.json")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Initialize a DataCollector instance
	dc := &mlagent.DataCollector{}

	// Call SetFetchStatusManager with the temporary file name
	dc.SetFetchStatusManager(tmpFile.Name())

	// Call GetFetchStatusManager and check if the returned manager is not nil
	manager := dc.GetFetchStatusManager()
	if manager == nil {
		t.Error("GetFetchStatusManager returned nil, expected a valid manager.")
	}
	os.Remove("status.json")
}
