package broker

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
)

func getValue(value, valueType string) (float64, error) {

	result := 0.0
	switch valueType {
	case common.ValueTypeFloat32:
		result, _ = strconv.ParseFloat(value, 32)
	case common.ValueTypeFloat64:
		result, _ = strconv.ParseFloat(value, 64)
	case common.ValueTypeUint8:
		valueUnint, _ := strconv.ParseUint(value, 10, 8)
		result = float64(valueUnint)
	case common.ValueTypeUint16:
		valueUnint, _ := strconv.ParseUint(value, 10, 16)
		result = float64(valueUnint)
	case common.ValueTypeUint32:
		valueUnint, _ := strconv.ParseUint(value, 10, 32)
		result = float64(valueUnint)
	case common.ValueTypeUint64:
		valueUnint, _ := strconv.ParseUint(value, 10, 64)
		result = float64(valueUnint)
	case common.ValueTypeInt8:
		valueInt, _ := strconv.ParseInt(value, 10, 8)
		result = float64(valueInt)
	case common.ValueTypeInt16:
		valueInt, _ := strconv.ParseInt(value, 10, 16)
		result = float64(valueInt)
	case common.ValueTypeInt32:
		valueInt, _ := strconv.ParseInt(value, 10, 32)
		result = float64(valueInt)
	case common.ValueTypeInt64:
		valueInt, _ := strconv.ParseInt(value, 10, 64)
		result = float64(valueInt)
	case common.ValueTypeBool:
		valueBool, _ := strconv.ParseBool(value)
		if valueBool {
			result = 1.0
		} else {
			result = 0.0
		}
	default:
		return result, fmt.Errorf("unsupported data type for anomaly detection: %s", valueType)
	}

	return result, nil

}

func validateEnvironmentVariables(env []string) error {
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, currentValue := parts[0], parts[1]
		if initialValue, ok := initialEnvVars[key]; ok && initialValue != currentValue {
			return fmt.Errorf("environment variable %s changed: was '%s', now '%s'", key, initialValue, currentValue)
		}
	}

	return nil
}

func RestartSelf() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	env := os.Environ()
	// Validate the environment variables that are going to be used for restart
	if err := validateEnvironmentVariables(env); err != nil {
		return fmt.Errorf("hedge-ml-broker environment variables validation failed: %v", err)
	}

	// self picks the current executable only, so executable can't be replaced
	// the code runs as a docker container or within K8s, so possibility to env var getting replaced is rare
	return syscall.Exec(self, []string{self}, env)
}

func buildCorrelationId(deviceName string, trainingConfigName string) string {
	return trainingConfigName + "_" + strings.ReplaceAll(deviceName, " ", "")
}

func float64ToByte(f float64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(f))
	return buf[:]
}

func getGroupName(groupNameWithLotNoAndRecordNo string) (string, int) {
	tokens := strings.Split(groupNameWithLotNoAndRecordNo, "__")
	groupName := tokens[0]
	lotNo := 0
	//currentRecordNumber := 0
	var err error
	if len(tokens) > 1 {
		lotNo, err = strconv.Atoi(tokens[1])
		if err != nil {
			lotNo = 0
		}
	}
	return groupName, lotNo
}

func calculatePercentageChange(actual, predicted float64) (float64, error) {
	if actual == 0 {
		return 0, fmt.Errorf("actual value cannot be zero when calculating percentage change")
	}
	percentageChange := ((predicted - actual) / math.Abs(actual)) * 100
	return percentageChange, nil
}

func parseInterfaceSliceToFloat64(input []interface{}) ([]float64, error) {
	result := make([]float64, len(input))
	for i, v := range input {
		val, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("value at index %d is not a float64, got %T", i, v)
		}
		result[i] = val
	}
	return result, nil
}
