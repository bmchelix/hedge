package helpers

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPreProcessor(t *testing.T) {
	t.Run("NewPreProcessor - Passed (nil Processor)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = nil
		testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{}

		testMlAlgoDefinition := buildTestMlAlgoDefinition()
		preProcessor := NewPreProcessor(
			u.AppService.LoggingClient(),
			testMlModelConfig,
			&testMlAlgoDefinition,
			false,
		)
		if preProcessor != nil {
			t.Error("Expected a nil PreProcessor, but got non-nil")
		}
	})
	t.Run("NewPreProcessor - Passed (non-nil Processor)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = nil
		testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{
			"profile1": {
				{Name: "feature1"},
				{Name: "feature2"},
				{Name: "feature3"},
			},
		}
		testMlModelConfig.MLDataSourceConfig.OutputPredictionCount = 0
		testMlModelConfig.MLDataSourceConfig.InputContextCount = 0

		testMlAlgoDefinition := buildTestMlAlgoDefinition()
		testMlAlgoDefinition.Type = TIMESERIES_ALGO_TYPE

		preProcessor := NewPreProcessor(
			u.AppService.LoggingClient(),
			testMlModelConfig,
			&testMlAlgoDefinition,
			false,
		)
		if preProcessor == nil {
			t.Error("Expected a non-nil PreProcessor, but got nil")
		}
	})
}

func TestPreProcessor_GetOutputFeaturesToIndex(t *testing.T) {
	t.Run("GetOutputFeaturesToIndex - Passed", func(t *testing.T) {
		expectedMap := map[string]int{
			"Feature1": 0,
			"Feature2": 1,
		}
		preProcessor := &PreProcessor{
			OutputFeaturesToIndex: expectedMap,
		}

		outputMap := preProcessor.GetOutputFeaturesToIndex()
		require.NotNil(t, outputMap)
		assert.Equal(t, expectedMap, outputMap)
	})
}

func TestPreProcessor_GetMLModelConfig(t *testing.T) {
	t.Run("GetMLModelConfig - Passed", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		preProcessor := &PreProcessor{
			MLModelConfig: buildTestMLModelConfig(),
		}

		preProcessor.MLModelConfig = testMlModelConfig
		result := preProcessor.GetMLModelConfig()
		if result != testMlModelConfig {
			t.Errorf("Expected MLModelConfig does not match the result")
		}
	})
}

func TestPreProcessor_GetMLAlgorithmDefinition(t *testing.T) {
	t.Run("GetMLAlgorithmDefinition - Passed", func(t *testing.T) {
		testMlAlgoDefinition := buildTestMlAlgoDefinition()
		preProcessor := &PreProcessor{
			MLAlgoDefinition: &testMlAlgoDefinition,
		}

		actualAlgoDef := preProcessor.GetMLAlgorithmDefinition()
		require.NotNil(t, actualAlgoDef)
		assert.Equal(t, &testMlAlgoDefinition, actualAlgoDef)
	})
}

func TestPreProcessor_BuildFeatureName(t *testing.T) {
	t.Run("BuildFeatureName - Passed", func(t *testing.T) {
		result := BuildFeatureName("profile1", "feature1")
		expected := "profile1#feature1"
		if result != expected {
			t.Errorf("Expected %q, but got %q", expected, result)
		}
	})
}

func TestPreProcessor_GetFeaturesToIndex(t *testing.T) {
	t.Run("GetFeaturesToIndex - Passed", func(t *testing.T) {
		preProcessor := &PreProcessor{}
		expectedFeaturesToIndex := map[string]int{
			"feature1": 0,
			"feature2": 1,
			"feature3": 2,
		}
		preProcessor.FeaturesToIndex = expectedFeaturesToIndex
		result := preProcessor.GetFeaturesToIndex()
		for featureName, expectedIndex := range expectedFeaturesToIndex {
			index, exists := result[featureName]
			if !exists {
				t.Errorf("Expected feature %q is missing in the result", featureName)
			} else if index != expectedIndex {
				t.Errorf("Expected index %d for feature %q, but got %d", expectedIndex, featureName, index)
			}
		}
	})
}

func TestPreProcessor_addTSDataElement(t *testing.T) {
	t.Run(
		"addTSDataElement - Passed (Adds multiple elements with sliding window)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.SupportSlidingWindow = true

			tsDataElements := []*data.TSDataElement{
				{DeviceName: "device1", MetricName: "metric1", Timestamp: 12345},
				{DeviceName: "device2", MetricName: "metric2", Timestamp: 12346},
			}
			expectedKeys := []string{
				"device1_metric1_12345",
				"device2_metric2_12346",
			}
			preProcessor := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				sampledData:   sync.Map{},
			}

			preProcessor.addTSDataElement(tsDataElements)
			for _, key := range expectedKeys {
				value, ok := preProcessor.sampledData.Load(key)
				require.True(t, ok, "Expected key not found in sampledData: %s", key)
				assert.NotNil(t, value, "Value for key %s should not be nil", key)
			}
		},
	)
	t.Run(
		"addTSDataElement - Passed (Adds single sample per interval without sliding window)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.SupportSlidingWindow = false

			tsDataElements := []*data.TSDataElement{
				{DeviceName: "device1", MetricName: "metric1", Timestamp: 12345},
			}
			expectedKey := "device1_metric1"
			preProcessor := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				sampledData:   sync.Map{},
			}

			preProcessor.addTSDataElement(tsDataElements)
			value, ok := preProcessor.sampledData.Load(expectedKey)
			require.True(t, ok, "Expected key not found in sampledData: %s", expectedKey)
			assert.NotNil(t, value, "Value for key %s should not be nil", expectedKey)
		},
	)
	t.Run("addTSDataElement - Passed (Handles empty input gracefully)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.SupportSlidingWindow = true

		tsDataElements := []*data.TSDataElement{}
		preProcessor := &PreProcessor{
			MLModelConfig: testMlModelConfig,
			sampledData:   sync.Map{},
		}

		preProcessor.addTSDataElement(tsDataElements)
		count := 0
		preProcessor.sampledData.Range(func(_, _ interface{}) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count, "sampledData should remain empty")
	})
}

func TestPreProcessor_GroupSamples(t *testing.T) {
	t.Run("GroupSamples - Passed (Groups samples by valid group keys)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName", "metricName"}
		testSampleData := sync.Map{}

		sampledData := []*data.TSDataElement{
			{
				GroupByMetaData: map[string]string{
					"deviceName": "device1",
					"metricName": "metric1",
				},
			},
			{
				GroupByMetaData: map[string]string{
					"deviceName": "device2",
					"metricName": "metric2",
				},
			},
		}

		expected := map[string][]*data.TSDataElement{
			"device1_metric1": {sampledData[0]},
			"device2_metric2": {sampledData[1]},
		}

		p := &PreProcessor{
			MLModelConfig: testMlModelConfig,
			sampledData:   testSampleData,
		}

		got := p.GroupSamples(sampledData)
		require.Equal(t, expected, got)
	})
	t.Run("GroupSamples - Passed (Skips nil data samples)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName"}
		testSampleData := sync.Map{}

		sampledData := []*data.TSDataElement{nil}

		expected := map[string][]*data.TSDataElement{}

		p := &PreProcessor{
			MLModelConfig: testMlModelConfig,
			sampledData:   testSampleData,
		}

		got := p.GroupSamples(sampledData)
		require.Equal(t, expected, got)
	})
	t.Run("GroupSamples - Passed (Skips samples missing required group keys)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName", "metricName"}
		testSampleData := sync.Map{}

		sampledData := []*data.TSDataElement{
			{GroupByMetaData: map[string]string{"deviceName": "device1"}}, // Missing "metricName"
		}
		expected := map[string][]*data.TSDataElement{}

		p := &PreProcessor{
			MLModelConfig: testMlModelConfig,
			sampledData:   testSampleData,
		}

		got := p.GroupSamples(sampledData)
		require.Equal(t, expected, got)
	})
	t.Run("GroupSamples - Passed (Handles sanitized grouping key values)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testMlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName", "metricName"}
		testSampleData := sync.Map{}

		sampledData := []*data.TSDataElement{
			{
				GroupByMetaData: map[string]string{
					"deviceName": "device 1", // Includes a space
					"metricName": "metric1",
				},
			},
		}

		expected := map[string][]*data.TSDataElement{
			"device_1_metric1": {sampledData[0]}, // Space replaced with underscore
		}

		p := &PreProcessor{
			MLModelConfig: testMlModelConfig,
			sampledData:   testSampleData,
		}

		got := p.GroupSamples(sampledData)
		require.Equal(t, expected, got)
	})
}

func TestPreProcessor_TakeSample(t *testing.T) {
	t.Run(
		"TakeSample - Passed (Returns true and samples when interval is met)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testSampleData := sync.Map{}
			currentTime := time.Now().Unix()

			sampledElements := []*data.TSDataElement{
				{DeviceName: "device1", MetricName: "metric1", Timestamp: 12345},
				{DeviceName: "device2", MetricName: "metric2", Timestamp: 12346},
			}

			p := &PreProcessor{
				MLModelConfig:         testMlModelConfig,
				sampledData:           testSampleData,
				lastSamplingStartTime: currentTime - 10,
				SamplingIntervalSecs:  5, // interval has passed
				logger:                u.AppService.LoggingClient(),
			}

			for _, elem := range sampledElements {
				p.sampledData.Store(elem.DeviceName+"_"+elem.MetricName, elem)
			}

			collected, samples := p.TakeSample(nil)
			require.True(t, collected, "Expected sampling to occur")
			require.NotNil(t, samples, "Expected samples to be collected")
			assert.ElementsMatch(
				t,
				sampledElements,
				samples,
				"Collected samples do not match expected elements",
			)
		},
	)
	t.Run("TakeSample - Passed (Returns false when interval has not been met)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testSampleData := sync.Map{}
		currentTime := time.Now().Unix()

		p := &PreProcessor{
			MLModelConfig:         testMlModelConfig,
			sampledData:           testSampleData,
			lastSamplingStartTime: currentTime - 2, // interval not met
			SamplingIntervalSecs:  5,
			logger:                u.AppService.LoggingClient(),
		}

		collected, samples := p.TakeSample(nil)
		require.False(t, collected, "Expected sampling not to occur")
		assert.Nil(t, samples, "Expected no samples to be collected")
	})
	t.Run("TakeSample - Passed (Handles empty input gracefully)", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig()
		testSampleData := sync.Map{}
		currentTime := time.Now().Unix()

		p := &PreProcessor{
			MLModelConfig:         testMlModelConfig,
			sampledData:           testSampleData,
			lastSamplingStartTime: currentTime - 10,
			SamplingIntervalSecs:  5,
			logger:                u.AppService.LoggingClient(),
		}

		collected, samples := p.TakeSample(nil)
		require.True(t, collected, "Expected sampling to occur")
		assert.Empty(t, samples, "Expected collected samples to be empty")
	})
}

func TestPreProcessor_AccumulateMultipleTimeseriesFeaturesSets(t *testing.T) {
	testReleaseCandidates := map[string][]data.InferenceData{
		"group1__12345": {{
			GroupName: "group1",
			Devices:   []string{"device1"},
			Tags:      map[string]interface{}{"tag1": "value1"},
			TimeStamp: 12345,
		}},
	}
	t.Run(
		"AccumulateMultipleTimeseriesFeaturesSets - Passed (Accumulates data and returns true when count is met)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLModelConfig.MLDataSourceConfig.InputContextCount = 3
			testReleaseCandidates["group1__12345"][0].Data = [][]interface{}{{1}, {2}, {3}}

			p := &PreProcessor{
				MLModelConfig:                 testMLModelConfig,
				sampledTimeseriesFeaturesSets: make(map[string]data.InferenceData),
				logger:                        u.AppService.LoggingClient(),
			}

			ready, accumulatedData := p.AccumulateMultipleTimeseriesFeaturesSets(
				testReleaseCandidates,
			)
			require.True(t, ready, "Expected data to be ready")
			require.NotNil(t, accumulatedData, "Expected accumulated data to be returned")
			assert.Equal(
				t,
				testReleaseCandidates["group1__12345"][0].Data,
				accumulatedData["group1"].Data,
				"Accumulated data does not match expected values",
			)
		},
	)
	t.Run(
		"AccumulateMultipleTimeseriesFeaturesSets - Passed (Keeps accumulating when count is not met)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLModelConfig.MLDataSourceConfig.InputContextCount = 5
			testReleaseCandidates["group1__12345"][0].Data = [][]interface{}{{1}, {2}}

			p := &PreProcessor{
				MLModelConfig:                 testMLModelConfig,
				sampledTimeseriesFeaturesSets: make(map[string]data.InferenceData),
				logger:                        u.AppService.LoggingClient(),
			}

			ready, accumulatedData := p.AccumulateMultipleTimeseriesFeaturesSets(
				testReleaseCandidates,
			)
			require.False(t, ready, "Expected data not to be ready")
			assert.Nil(t, accumulatedData, "Expected no accumulated data to be returned")
			assert.Contains(
				t,
				p.sampledTimeseriesFeaturesSets,
				"group1",
				"Expected data to be accumulated in sampledTimeseriesFeaturesSets",
			)
			assert.Equal(
				t,
				testReleaseCandidates["group1__12345"][0].Data,
				p.sampledTimeseriesFeaturesSets["group1"].Data,
				"Accumulated data does not match expected values",
			)
		},
	)
	t.Run(
		"AccumulateMultipleTimeseriesFeaturesSets - Passed (Appends data to existing group)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLModelConfig.MLDataSourceConfig.InputContextCount = 5
			testReleaseCandidates["group1__12345"][0].Data = [][]interface{}{{3}, {4}}

			p := &PreProcessor{
				MLModelConfig: testMLModelConfig,
				sampledTimeseriesFeaturesSets: map[string]data.InferenceData{
					"group1": {
						GroupName: "group1",
						Devices:   []string{"device1"},
						Data:      [][]interface{}{{1}, {2}},
						Tags:      map[string]interface{}{"tag1": "value1"},
						TimeStamp: 12345,
					},
				},
				logger: u.AppService.LoggingClient(),
			}

			ready, accumulatedData := p.AccumulateMultipleTimeseriesFeaturesSets(
				testReleaseCandidates,
			)
			require.False(t, ready, "Expected data not to be ready")
			assert.Nil(t, accumulatedData, "Expected no accumulated data to be returned")
			assert.Contains(
				t,
				p.sampledTimeseriesFeaturesSets,
				"group1",
				"Expected data to be accumulated in sampledTimeseriesFeaturesSets",
			)
			assert.Equal(
				t,
				[][]interface{}{{1}, {2}, {3}, {4}},
				p.sampledTimeseriesFeaturesSets["group1"].Data,
				"Accumulated data does not match expected values",
			)
		},
	)
}

func TestPreProcessor_TransformToFeature(t *testing.T) {
	t.Run(
		"TransformToFeature - Passed (Complete feature set with initialized data)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLModelConfig.MLDataSourceConfig.GroupOrJoinKeys = []string{"deviceName"}
			testMLAlgoDefinition := &config.MLAlgorithmDefinition{
				Type:                      TIMESERIES_ALGO_TYPE,
				GroupByAttributesRequired: true,
			}

			featuresToIndex := map[string]int{
				config.TIMESTAMP_FEATURE_COL_NAME: 0,
				"deviceName":                      1,
				"profile1#metric1":                2,
			}

			sampledData := []*data.TSDataElement{
				{
					Profile:         "profile1",
					MetricName:      "metric1",
					Value:           100,
					Timestamp:       1234567890,
					GroupByMetaData: map[string]string{"deviceName": "device1"},
					DeviceName:      "device1",
				},
			}

			expectedFeatureSet := map[int64][]interface{}{
				1234567890: {int64(1234567890), "device1", 100},
			}

			p := &PreProcessor{
				MLModelConfig:           testMLModelConfig,
				MLAlgoDefinition:        testMLAlgoDefinition,
				FeaturesToIndex:         featuresToIndex,
				IsTrainingContext:       false,
				PreviousFeaturesByGroup: make(map[string][]interface{}),
				logger:                  u.AppService.LoggingClient(),
			}

			featureSet, featuresComplete := p.TransformToFeature("group1", sampledData)
			require.True(t, featuresComplete, "Expected features to be complete")
			require.NotNil(t, featureSet, "Expected non-nil feature set")
			assert.Equal(
				t,
				expectedFeatureSet,
				featureSet,
				"Feature set does not match expected values",
			)
		},
	)
	t.Run(
		"TransformToFeature - Failed (Incomplete feature set due to missing data)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLAlgoDefinition := &config.MLAlgorithmDefinition{
				GroupByAttributesRequired: false,
			}
			featuresToIndex := map[string]int{
				"deviceName":                      1,
				"metric1":                         2,
				config.TIMESTAMP_FEATURE_COL_NAME: 0,
			}
			sampledData := []*data.TSDataElement{
				{Profile: "profile1", MetricName: "metric2", Value: 200, Timestamp: 1234567890},
			}

			p := &PreProcessor{
				MLModelConfig:           testMLModelConfig,
				MLAlgoDefinition:        testMLAlgoDefinition,
				FeaturesToIndex:         featuresToIndex,
				IsTrainingContext:       true,
				PreviousFeaturesByGroup: make(map[string][]interface{}),
				logger:                  u.AppService.LoggingClient(),
			}

			featureSet, featuresComplete := p.TransformToFeature("group1", sampledData)
			require.False(t, featuresComplete, "Expected features to be incomplete")
			assert.Empty(t, featureSet, "Expected empty feature set for missing data")
		},
	)
	t.Run(
		"TransformToFeature - Passed (Sliding window updates previous features)",
		func(t *testing.T) {
			testMLModelConfig := buildTestMLModelConfig()
			testMLModelConfig.MLDataSourceConfig.SupportSlidingWindow = true
			testMLAlgoDefinition := &config.MLAlgorithmDefinition{
				GroupByAttributesRequired: false,
			}
			featuresToIndex := map[string]int{
				"deviceName":                      1,
				"metric1":                         2,
				config.TIMESTAMP_FEATURE_COL_NAME: 0,
			}
			sampledData := []*data.TSDataElement{
				{
					Profile:         "profile1",
					MetricName:      "metric1",
					Value:           150,
					Timestamp:       1234567891,
					GroupByMetaData: map[string]string{"deviceName": "device2"},
				},
			}

			p := &PreProcessor{
				MLModelConfig:     testMLModelConfig,
				MLAlgoDefinition:  testMLAlgoDefinition,
				FeaturesToIndex:   featuresToIndex,
				IsTrainingContext: true,
				PreviousFeaturesByGroup: map[string][]interface{}{
					"group1": {1234567890, "device1", 100},
				},
				logger: u.AppService.LoggingClient(),
			}

			featureSet, featuresComplete := p.TransformToFeature("group1", sampledData)
			require.True(t, featuresComplete, "Expected features to be complete")
			require.NotNil(t, featureSet, "Expected non-nil feature set")
			assert.Contains(
				t,
				p.PreviousFeaturesByGroup,
				"group1",
				"Expected updated previous features for group",
			)
		},
	)
	t.Run("TransformToFeature - Passed (Handles empty input gracefully)", func(t *testing.T) {
		testMLModelConfig := buildTestMLModelConfig()
		testMLAlgoDefinition := &config.MLAlgorithmDefinition{}
		featuresToIndex := map[string]int{
			"deviceName": 1,
		}
		sampledData := []*data.TSDataElement{}

		p := &PreProcessor{
			MLModelConfig:           testMLModelConfig,
			MLAlgoDefinition:        testMLAlgoDefinition,
			FeaturesToIndex:         featuresToIndex,
			IsTrainingContext:       true,
			PreviousFeaturesByGroup: make(map[string][]interface{}),
			logger:                  u.AppService.LoggingClient(),
		}

		featureSet, featuresComplete := p.TransformToFeature("group1", sampledData)
		require.False(t, featuresComplete, "Expected features to be incomplete")
		assert.Empty(t, featureSet, "Expected empty feature set for empty input")
	})
}

func TestPreProcessor_GetOutputFeaturesToValues(t *testing.T) {
	t.Run(
		"GetOutputFeaturesToValues - Passed (Extracts output features correctly)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{
				"profile1": {
					{Name: "feature1", IsOutput: true},
					{Name: "feature2", IsOutput: false},
				},
			}
			sampledData := []*data.TSDataElement{
				{Profile: "profile1", MetricName: "feature1", Value: 42},
				{Profile: "profile1", MetricName: "feature2", Value: 100},
			}
			expectedOutput := map[string]interface{}{
				"feature1": 42,
			}

			p := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				logger:        u.AppService.LoggingClient(),
			}

			output := p.GetOutputFeaturesToValues(sampledData)
			require.NotNil(t, output, "Expected output to be non-nil")
			assert.Equal(t, expectedOutput, output, "Output features do not match expected values")
		},
	)
	t.Run(
		"GetOutputFeaturesToValues - Passed (Handles missing profiles gracefully)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{
				"profile1": {
					{Name: "feature1", IsOutput: true},
				},
			}
			sampledData := []*data.TSDataElement{
				{Profile: "profile2", MetricName: "feature3", Value: NotInitializedValue},
			}
			expectedOutput := map[string]interface{}{}

			p := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				logger:        u.AppService.LoggingClient(),
			}

			output := p.GetOutputFeaturesToValues(sampledData)
			require.NotNil(t, output, "Expected output to be non-nil")
			assert.Equal(
				t,
				expectedOutput,
				output,
				"Output should be empty when profiles are missing",
			)
		},
	)

	t.Run(
		"GetOutputFeaturesToValues - Passed (Ignores features that are not outputs)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{
				"profile1": {
					{Name: "feature1", IsOutput: false},
				},
			}
			sampledData := []*data.TSDataElement{
				{Profile: "profile1", MetricName: "feature1", Value: 123},
			}
			expectedOutput := map[string]interface{}{}

			p := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				logger:        u.AppService.LoggingClient(),
			}

			output := p.GetOutputFeaturesToValues(sampledData)
			require.NotNil(t, output, "Expected output to be non-nil")
			assert.Equal(
				t,
				expectedOutput,
				output,
				"Output should be empty for non-output features",
			)
		},
	)
	t.Run(
		"GetOutputFeaturesToValues - Passed (Handles empty input gracefully)",
		func(t *testing.T) {
			testMlModelConfig := buildTestMLModelConfig()
			testMlModelConfig.MLDataSourceConfig.FeaturesByProfile = map[string][]config.Feature{}

			sampledData := []*data.TSDataElement{}
			expectedOutput := map[string]interface{}{}

			p := &PreProcessor{
				MLModelConfig: testMlModelConfig,
				logger:        u.AppService.LoggingClient(),
			}

			output := p.GetOutputFeaturesToValues(sampledData)
			require.NotNil(t, output, "Expected output to be non-nil")
			assert.Equal(t, expectedOutput, output, "Output should be empty for empty input")
		},
	)
}

func TestPreProcessor_SetEmptyStringValueForExternalFeatures(t *testing.T) {
	t.Run(
		"setEmptyStringValueForExternalFeatures - Passed (Sets empty string for external features)",
		func(t *testing.T) {
			featuresToIndex := map[string]int{
				"external#externalFeature1": 3,
			}
			features := []interface{}{NotInitializedValue, NotInitializedValue, NotInitializedValue, NotInitializedValue}

			p := &PreProcessor{
				FeaturesToIndex: featuresToIndex,
				MLModelConfig: &config.MLModelConfig{
					MLDataSourceConfig: config.MLModelDataConfig{
						ExternalFeatures: []config.Feature{
							{Type: "CATEGORICAL",
								Name:                   "externalFeature1",
								IsOutput:               true,
								IsInput:                false,
								FromExternalDataSource: true,
							},
						},
					},
				},
			}

			expectedFeatures := []interface{}{NotInitializedValue, NotInitializedValue, NotInitializedValue, ""}

			p.setEmptyStringValueForExternalFeatures(features)
			require.Equal(
				t,
				expectedFeatures,
				features,
				"Features array does not match expected values after processing",
			)
		},
	)
	t.Run(
		"setEmptyStringValueForExternalFeatures - Passed (Handles empty external features list)",
		func(t *testing.T) {
			featuresToIndex := map[string]int{
				"profile#value1": 0,
				"profile#value2": 1,
				"profile#value3": 2,
			}
			features := []interface{}{"value1", "value2", "value3"}

			p := &PreProcessor{
				FeaturesToIndex: featuresToIndex,
				MLModelConfig: &config.MLModelConfig{
					MLDataSourceConfig: config.MLModelDataConfig{
						ExternalFeatures: []config.Feature{},
					},
				},
			}

			expectedFeatures := []interface{}{"value1", "value2", "value3"}
			p.setEmptyStringValueForExternalFeatures(features)
			require.Equal(
				t,
				expectedFeatures,
				features,
				"Features array should remain unchanged when external features list is empty",
			)
		},
	)
}

func TestPreProcessor_generateOutputFeatureMapping(t *testing.T) {
	mlModelConfig := &config.MLModelConfig{
		MLDataSourceConfig: config.MLModelDataConfig{
			GroupOrJoinKeys: []string{"deviceName"},
			FeaturesByProfile: map[string][]config.Feature{
				"WindTurbine": {
					{Name: "TurbinePower", IsInput: true, IsOutput: true, FromExternalDataSource: false},
					{Name: "RotorSpeed", IsInput: true, IsOutput: false, FromExternalDataSource: false},
					{Name: "WindSpeed", IsInput: true, IsOutput: true, FromExternalDataSource: false},
				},
			},
		},
	}
	expectedMapping := map[string]int{
		"Timestamp":                0,
		"deviceName":               1,
		"WindTurbine#TurbinePower": 2,
		"WindTurbine#WindSpeed":    3,
	}
	preProcessor := &PreProcessor{
		MLModelConfig: mlModelConfig,
	}
	actualMapping := preProcessor.generateOutputFeatureMapping()
	if !reflect.DeepEqual(actualMapping, expectedMapping) {
		t.Errorf("generateOutputFeatureMapping() = %v, want %v", actualMapping, expectedMapping)
	}
}
