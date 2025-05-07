package broker

import (
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/stretchr/testify/assert"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestUtils_RestartSelf_FailOnEnvValidation(t *testing.T) {
	// Skip test on Windows since the function behaves differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows.")
	}
	originalEnv := os.Environ()
	initialEnvVars = map[string]string{
		"ENVIRONMENT":                         "prod",
		"SERVICE_HOST":                        "wrong-host",
		"SECRETSTORE_DISABLESCRUBSECRETSFILE": "true",
	}

	os.Clearenv()
	os.Setenv("ENVIRONMENT", "prod")
	os.Setenv("SERVICE_HOST", "change-value")

	err := RestartSelf()
	if err == nil || !strings.Contains(err.Error(), "environment variables validation failed") {
		t.Errorf("Expected environment validation failure, got %v", err)
	}
	restoreEnv(originalEnv)

	initialEnvVars = nil
}

// restoreEnv restores the environment variables
func restoreEnv(env []string) {
	os.Clearenv()
	for _, e := range env {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			os.Setenv(pair[0], pair[1])
		}
	}
}

func TestUtils_GetValue(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		valueType    string
		expected     float64
		expectingErr bool
	}{
		// Float cases
		{"Valid Float32", "12.34", common.ValueTypeFloat32, 12.34, false},
		{"Valid Float64", "56.78", common.ValueTypeFloat64, 56.78, false},
		{"Invalid Float32", "invalid", common.ValueTypeFloat32, 0.0, false},

		// Uint cases
		{"Valid Uint8", "255", common.ValueTypeUint8, 255, false},
		{"Valid Uint16", "65535", common.ValueTypeUint16, 65535, false},
		{"Valid Uint32", "4294967295", common.ValueTypeUint32, 4294967295, false},
		{"Valid Uint64", "18446744073709551615", common.ValueTypeUint64, 18446744073709551615, false},
		{"Invalid Uint8", "invalid", common.ValueTypeUint8, 0.0, false},

		// Int cases
		{"Valid Int8", "-128", common.ValueTypeInt8, -128, false},
		{"Valid Int16", "-32768", common.ValueTypeInt16, -32768, false},
		{"Valid Int32", "-2147483648", common.ValueTypeInt32, -2147483648, false},
		{"Valid Int64", "-9223372036854775808", common.ValueTypeInt64, -9223372036854775808, false},
		{"Invalid Int8", "invalid", common.ValueTypeInt8, 0.0, false},

		// Bool cases
		{"Valid Bool true", "true", common.ValueTypeBool, 1.0, false},
		{"Valid Bool false", "false", common.ValueTypeBool, 0.0, false},
		{"Invalid Bool", "invalid", common.ValueTypeBool, 0.0, false},

		// Unsupported type case
		{"Unsupported ValueType", "1234", common.ValueTypeBinary, 0.0, true}, // Handle unsupported value type
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getValue(tt.value, tt.valueType)
			if tt.expectingErr {
				assert.Error(t, err, "Expected an error but got none")
				return
			}
			assert.NoError(t, err, "Unexpected error occurred")

			// Use assert.Equal for invalid input and boolean cases to avoid epsilon comparison issues
			if tt.expectingErr || tt.valueType == common.ValueTypeBool || tt.value == "invalid" {
				assert.Equal(t, tt.expected, got, "Expected %v, got %v", tt.expected, got)
			} else if tt.valueType == common.ValueTypeFloat32 || tt.valueType == common.ValueTypeFloat64 {
				// Use assert.InEpsilon for float comparisons to account for precision differences
				assert.InEpsilon(t, tt.expected, got, 0.0001, "Expected %v, got %v", tt.expected, got)
			} else {
				// Use assert.Equal for other value types
				assert.Equal(t, tt.expected, got, "Expected %v, got %v", tt.expected, got)
			}
		})
	}
}
func TestUtils_ValidateEnvironmentVariables(t *testing.T) {
	initialEnvVars = map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}
	env1 := []string{
		"KEY1=value1",
		"KEY2=value2",
		"KEY3=value3",
	}
	env2 := []string{
		"KEY1=value1",
		"KEY2=newValue2", // This will trigger the error in the function
		"KEY3=value3",
	}
	env3 := []string{
		"KEY1=newValue1", // This will trigger the error in the function
		"KEY2=value2",
		"KEY3=value3",
	}
	env4 := []string{
		"KEY1=value1",
		"KEY2=value2",
		"KEY4=newValue4",
	}
	env5 := []string{
		"KEY1=value1",
		"KEY2=value2",
		"INVALID_FORMAT",
	}
	env6 := []string{
		"KEY1=newValue1",
		"KEY2=newValue2",
		"KEY3=newValue3",
	}
	tests := []struct {
		name          string
		env           []string
		expectedError string
	}{
		{"ValidateEnvironmentVariables - Valid environment variables - no changes", env1, ""},
		{"ValidateEnvironmentVariables - Environment variable KEY2 changed", env2, "environment variable KEY2 changed: was 'value2', now 'newValue2'"},
		{"ValidateEnvironmentVariables - Environment variable KEY1 changed", env3, "environment variable KEY1 changed: was 'value1', now 'newValue1'"},
		{"ValidateEnvironmentVariables - Environment variable not in initialEnvVars", env4, ""},
		{"ValidateEnvironmentVariables - Environment variable in invalid format", env5, ""},
		{"ValidateEnvironmentVariables - Empty environment variables", []string{}, ""},
		{"ValidateEnvironmentVariables - All environment variables changed", env6, "environment variable KEY1 changed: was 'value1', now 'newValue1'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentVariables(tt.env)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func Test_float64ToByte(t *testing.T) {
	testCases := []struct {
		input    float64
		expected []byte
	}{
		{0.0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{1.23, []byte{63, 243, 174, 20, 122, 225, 71, 174}},
		{-42.42, []byte{192, 69, 53, 194, 143, 92, 40, 246}},
		{math.Inf(1), []byte{127, 240, 0, 0, 0, 0, 0, 0}},
		{math.Inf(-1), []byte{255, 240, 0, 0, 0, 0, 0, 0}},
		{math.NaN(), []byte{127, 248, 0, 0, 0, 0, 0, 1}},
	}
	for _, testCase := range testCases {
		result := float64ToByte(testCase.input)
		if !areByteSlicesEqual(result, testCase.expected) {
			t.Errorf("For input %v, expected %v, but got %v", testCase.input, testCase.expected, result)
		}
	}
}

func areByteSlicesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Test_calculatePercentageChange(t *testing.T) {
	tests := []struct {
		name         string
		actual       float64
		predicted    float64
		expected     float64
		expectingErr bool
	}{
		{"calculatePercentageChange - Positive Increase", 100, 120, 20, false},
		{"calculatePercentageChange - Positive Decrease", 100, 80, -20, false},
		{"calculatePercentageChange- No Change", 100, 100, 0, false},
		{"calculatePercentageChange - Zero Actual", 0, 100, 0, true},
		{"calculatePercentageChange - Negative Actual", -100, -80, 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculatePercentageChange(tt.actual, tt.predicted)

			if tt.expectingErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expected, result, 0.0001, "percentage change mismatch")
			}
		})
	}
}

func Test_parseInterfaceSliceToFloat64(t *testing.T) {
	tests := []struct {
		name         string
		input        []interface{}
		expected     []float64
		expectingErr bool
		expectedErr  string
	}{
		{"parseInterfaceSliceToFloat64 - Passed (valid slice of float64)", []interface{}{1.23, 4.56, 7.89}, []float64{1.23, 4.56, 7.89}, false, ""},
		{"parseInterfaceSliceToFloat64 - Failed (mixed types in slice)", []interface{}{1.23, "not a float", 7.89}, nil, true, "value at index 1 is not a float64, got string"},
		{"parseInterfaceSliceToFloat64 - Passed (empty slice)", []interface{}{}, []float64{}, false, ""},
		{"parseInterfaceSliceToFloat64 - Passed (single valid float64)", []interface{}{42.42}, []float64{42.42}, false, ""},
		{"parseInterfaceSliceToFloat64 - Failed (all invalid types)", []interface{}{"string", true, []int{1, 2, 3}}, nil, true, "value at index 0 is not a float64, got string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInterfaceSliceToFloat64(tt.input)

			if tt.expectingErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result, fmt.Sprintf("expected %v, got %v", tt.expected, result))
			}
		})
	}
}
