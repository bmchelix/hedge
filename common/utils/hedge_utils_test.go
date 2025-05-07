/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package utils

import (
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"encoding/base64"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	mocks1 "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"math"
	"net/http"
	"testing"
)

func TestHedgeUtils_ParseSimpleValueToFloat64(t *testing.T) {
	tolerance := 0.0001 // Define a small tolerance level

	testCases := []struct {
		name      string
		valueType string
		value     string
		expected  float64
		wantErr   bool
	}{
		{"uint8 valid", common.ValueTypeUint8, "100", 100, false},
		{"uint8 invalid", common.ValueTypeUint8, "a", 0, true},
		{"uint16 valid", common.ValueTypeUint16, "1000", 1000, false},
		{"uint32 valid", common.ValueTypeUint32, "30000", 30000, false},
		{"uint64 valid", common.ValueTypeUint64, "50000", 50000, false},
		{"int8 valid", common.ValueTypeInt8, "-100", -100, false},
		{"int8 invalid", common.ValueTypeInt8, "abc", 0, true},
		{"int16 valid", common.ValueTypeInt16, "-1000", -1000, false},
		{"int32 valid", common.ValueTypeInt32, "-30000", -30000, false},
		{"int64 valid", common.ValueTypeInt64, "-50000", -50000, false},
		{"float32 valid", common.ValueTypeFloat32, "123.456", 123.456, false},
		{"float64 valid", common.ValueTypeFloat64, "123456.789", 123456.789, false},
		{"float64 invalid", common.ValueTypeFloat64, "xyz", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSimpleValueToFloat64(tc.valueType, tc.value)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseSimpleValueToFloat64() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && math.Abs(got-tc.expected) > tolerance {
				t.Errorf("ParseSimpleValueToFloat64() got = %v, want %v within a tolerance of %v", got, tc.expected, tolerance)
			}
		})
	}
}

func TestHedgeUtils_Contains(t *testing.T) {
	t.Run("Element exists in slice", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		element := "banana"
		expected := true

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
	t.Run("Element does not exist in slice", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		element := "orange"
		expected := false

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
	t.Run("Slice is empty", func(t *testing.T) {
		slice := []string{}
		element := "banana"
		expected := false

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
	t.Run("Element is an empty string", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		element := ""
		expected := false

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
	t.Run("Slice contains only the element", func(t *testing.T) {
		slice := []string{"banana"}
		element := "banana"
		expected := true

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
	t.Run("Case-sensitive match", func(t *testing.T) {
		slice := []string{"Apple", "Banana", "Cherry"}
		element := "banana"
		expected := false

		result := Contains(slice, element)
		assert.Equal(t, expected, result)
	})
}

func TestHedgeUtils_GetUserIdFromHeader(t *testing.T) {
	t.Run("GetUserIdFromHeader - Passed (Header contains user ID directly)", func(t *testing.T) {
		svc := setupServiceMock().AppService
		svc.On("GetAppSetting", "UserId_header").Return("X-Credential-Identifier", nil)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("X-Credential-Identifier", "test-user-id")

		userID, err := GetUserIdFromHeader(req, svc)
		assert.NoError(t, err)
		assert.Equal(t, "test-user-id", userID)
	})
	t.Run("GetUserIdFromHeader - Passed (Header name from app settings not found, fallback headers used)", func(t *testing.T) {
		svc := setupServiceMock().AppService
		svc.On("GetAppSetting", "UserId_header").Return("", errors.New("dummy error"))

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("helix_sso_uid", "fallback-user-id")

		userID, err := GetUserIdFromHeader(req, svc)
		assert.NoError(t, err)
		assert.Equal(t, "fallback-user-id", userID)
	})
	t.Run("GetUserIdFromHeader - Passed (Header contains authorization, user ID extracted from basic auth)", func(t *testing.T) {
		svc := setupServiceMock().AppService
		svc.On("GetAppSetting", "UserId_header").Return("", errors.New("dummy error"))

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("extracted-user-id:password")))

		userID, err := GetUserIdFromHeader(req, svc)
		assert.NoError(t, err)
		assert.Equal(t, "extracted-user-id", userID)
	})
	t.Run("GetUserIdFromHeader - Failed (Header missing, no user ID found)", func(t *testing.T) {
		svc := setupServiceMock().AppService
		svc.On("GetAppSetting", "UserId_header").Return("", errors.New("dummy error"))

		req, _ := http.NewRequest("GET", "/", nil)

		userID, err := GetUserIdFromHeader(req, svc)
		assert.Error(t, err)
		assert.Empty(t, userID)
	})
}

func setupServiceMock() *utils.HedgeMockUtils {
	hedgeMockUtils := new(utils.HedgeMockUtils)
	mockLogger := &mocks1.LoggingClient{}
	mockAppService := &mocks.ApplicationService{}
	hedgeMockUtils.AppService = mockAppService
	mockAppService.On("LoggingClient").Return(mockLogger)
	mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything).Return()
	return hedgeMockUtils
}

func TestToFloat64(t *testing.T) {
	t.Run("ToFloat64 - Success - float64 value", func(t *testing.T) {
		value := 42.5
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})
	t.Run("ToFloat64 - Success - float32 value", func(t *testing.T) {
		value := float32(42.5)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - int value", func(t *testing.T) {
		value := 42
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - int8 value", func(t *testing.T) {
		value := int8(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - int16 value", func(t *testing.T) {
		value := int16(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - int32 value", func(t *testing.T) {
		value := int32(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - int64 value", func(t *testing.T) {
		value := int64(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - uint value", func(t *testing.T) {
		value := uint(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - uint8 value", func(t *testing.T) {
		value := uint8(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - uint16 value", func(t *testing.T) {
		value := uint16(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - uint32 value", func(t *testing.T) {
		value := uint32(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Success - uint64 value", func(t *testing.T) {
		value := uint64(42)
		expected := float64(value)
		result, err := ToFloat64(value)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})
	t.Run("ToFloat64 - Failure - unsupported type", func(t *testing.T) {
		value := "42.5"
		result, err := ToFloat64(value)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type")
		assert.Equal(t, float64(0), result)
	})
}
