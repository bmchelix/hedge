/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package utils

import (
	hedgeErrors "hedge/common/errors"
	"encoding/base64"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"net/http"
	"strconv"
	"strings"
)

// The below code was picked from edgex source code
func ParseSimpleValueToFloat64(valueType string, value string) (valFloat float64, err error) {
	var val interface{}
	switch valueType {
	case common.ValueTypeUint8:
		val, err = strconv.ParseUint(value, 10, 8)
		if err == nil {
			return float64(val.(uint64)), nil
		}
	case common.ValueTypeUint16:
		val, err = strconv.ParseUint(value, 10, 16)
		if err == nil {
			return float64(val.(uint64)), nil
		}
	case common.ValueTypeUint32:
		val, err = strconv.ParseUint(value, 10, 32)
		if err == nil {
			return float64(val.(uint64)), nil
		}
	case common.ValueTypeUint64:
		val, err = strconv.ParseUint(value, 10, 64)
		if err == nil {
			return float64(val.(uint64)), nil
		}
	case common.ValueTypeInt8:
		val, err = strconv.ParseInt(value, 10, 8)
		if err == nil {
			return float64(val.(int64)), nil
		}
	case common.ValueTypeInt16:
		val, err = strconv.ParseInt(value, 10, 16)
		if err == nil {
			return float64(val.(int64)), nil
		}
	case common.ValueTypeInt32:
		val, err = strconv.ParseInt(value, 10, 32)
		if err == nil {
			return float64(val.(int64)), nil
		}
	case common.ValueTypeInt64:
		val, err = strconv.ParseInt(value, 10, 64)
		if err == nil {
			return float64(val.(int64)), nil
		}

	case common.ValueTypeFloat32:
		val, err = strconv.ParseFloat(value, 32)
		if err == nil {
			return val.(float64), nil
		}
	case common.ValueTypeFloat64:
		val, err = strconv.ParseFloat(value, 64)
		if err == nil {
			return val.(float64), nil
		}
	}

	return 0, err
}

func ToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", value)
	}
	return 0, nil
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func GetUserIdFromHeader(req *http.Request, service interfaces.ApplicationService) (string, hedgeErrors.HedgeError) {
	userIdHeaderName, err := service.GetAppSetting("UserId_header")
	var userId string
	if err != nil {
		userIdHeaderName = "X-Credential-Identifier"
		userId = req.Header.Get("X-Credential-Identifier")
		if userId == "" {
			userIdHeaderName = "X-Consumer-Username"
			userId = req.Header.Get("X-Consumer-Username")
			if userId == "" {
				userIdHeaderName = "helix_sso_uid"
				userId = req.Header.Get("helix_sso_uid")
			}
		}
	} else {
		userId = req.Header.Get(userIdHeaderName)
	}

	switch userId {
	case "":
		if req.Header.Get("Authorization") != "" {
			tmp, _ := base64.StdEncoding.DecodeString(strings.Split(req.Header.Get("Authorization"), " ")[1])
			userId := strings.Split(string(tmp), ":")[0]
			userIdHeaderName = "Authorization"
			service.LoggingClient().Infof("Retrieved userId '%s' from request header '%s'", userId, userIdHeaderName)
			return userId, nil
		}
	default:
		service.LoggingClient().Infof("Retrieved userId '%s' from request header '%s'", userId, userIdHeaderName)
		return userId, nil
	}

	return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, "HTTP header with userid not found")
}
