/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/common/client"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"io"
	"net/http"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
)

type HTTPSenderInterface interface {
	HTTPPost(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	HTTPPatch(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	HTTPPut(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	HTTPDelete(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	HTTPGet(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	HTTPSend(ctx interfaces.AppFunctionContext, data interface{}, method string) (bool, interface{})
	GetURL() string
	SetURL(url string)
	GetMimeType() string
	SetPersistOnError(updatedPersistOnErrorValue bool)
	GetPersistOnError() bool
}

type HTTPSender struct {
	URL            string
	MimeType       string
	PersistOnError bool
	Headers        map[string]string
	Cookie         *http.Cookie
}

type ExportData struct {
	payload          []byte
	storeForwardData []byte
}

func NewExportData(payload []byte, storeForwardData []byte) ExportData {
	return ExportData{
		payload:          payload,
		storeForwardData: storeForwardData,
	}
}

func ConvertToByteArray(obj interface{}) []byte {
	byteArray, err := json.Marshal(obj)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}
	return byteArray
}

func NewDummyCookie() http.Cookie {
	return http.Cookie{
		Name:     "",
		Value:    "",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode, // Set SameSite to StrictMode for enhanced security
	}
}

// HttpSender for store and forward
func NewHTTPSender(url string, mimeType string, persistOnError bool, headers map[string]string, cookie *http.Cookie) *HTTPSender {
	return &HTTPSender{
		URL:            url,
		MimeType:       mimeType,
		PersistOnError: persistOnError,
		Headers:        headers,
		Cookie:         cookie,
	}
}

func LogErrorResponseFromService(loggerClient logger.LoggingClient, requestUrl string, errorMessage string, response interface{}) {
	completeErrorMessage := fmt.Sprintf("%s: Request to %s failed or returned error", errorMessage, requestUrl)

	if response == nil {
		loggerClient.Errorf("%s: response data is nil", completeErrorMessage)
		return
	}

	responseObj, ok := response.(*http.Response)
	if !ok || responseObj == nil {
		loggerClient.Errorf("%s: response is either nil or not of type http.Response", completeErrorMessage)
		return
	}

	body := responseObj.Body
	defer body.Close()
	errResponseBytes, err := io.ReadAll(body)
	if err != nil {
		loggerClient.Errorf("%s: error response code %v: response body is unavailable for logging", completeErrorMessage, responseObj.StatusCode)
		return
	}
	var errResponse map[string]interface{}
	err = json.Unmarshal(errResponseBytes, &errResponse)
	if err != nil {
		loggerClient.Errorf("%s: error response code %v: response body is unavailable for logging", completeErrorMessage, responseObj.StatusCode)
		return
	}
	loggerClient.Errorf("%s: error response code %v: response body %v", completeErrorMessage, responseObj.StatusCode, errResponse)
}

func (sender *HTTPSender) HTTPPost(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	return sender.HTTPSend(ctx, data, http.MethodPost)
}
func (sender *HTTPSender) HTTPPatch(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	return sender.HTTPSend(ctx, data, http.MethodPatch)
}
func (sender *HTTPSender) HTTPPut(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	return sender.HTTPSend(ctx, data, http.MethodPut)
}
func (sender *HTTPSender) HTTPDelete(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	return sender.HTTPSend(ctx, data, http.MethodDelete)
}
func (sender *HTTPSender) HTTPGet(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	return sender.HTTPSend(ctx, data, http.MethodGet)
}
func (sender *HTTPSender) HTTPSend(ctx interfaces.AppFunctionContext, data interface{}, method string) (bool, interface{}) {
	lc := ctx.LoggingClient()
	lc.Debugf("HTTP Exporting in pipeline '%s'", ctx.PipelineId())

	var err error
	var req *http.Request
	var exportData ExportData
	if data == nil {
		lc.Debugf("HTTP request data is nil")
		req, err = http.NewRequest(method, sender.URL, nil)
	} else {
		ok := false
		exportData, ok = data.(ExportData)
		if !ok {
			lc.Errorf("Invalid data received. Failed when casting to ExportData")
			return false, errors.New("invalid data received. Failed when casting to ExportData")
		}

		req, err = http.NewRequest(method, sender.URL, bytes.NewReader(exportData.payload))
		if err != nil {
			if sender.PersistOnError {
				lc.Debugf("Pipeline '%s': Saving data", ctx.PipelineId())
				ctx.SetRetryData(exportData.storeForwardData)
			}
			return false, err
		}
	}

	req.Header.Set("Content-Type", sender.MimeType)
	if sender.Headers != nil {
		for headerName, value := range sender.Headers {
			req.Header.Set(headerName, value)
		}
	}
	if sender.Cookie != nil && sender.Cookie.Name != "" {
		req.AddCookie(sender.Cookie)
	}

	lc.Debugf("POSTing data to %s in pipeline '%s'", sender.URL, ctx.PipelineId())
	response, err := client.Client.Do(req)

	responseReceived := response != nil
	var badStatus bool
	if responseReceived {
		badStatus = response.StatusCode < 200 || response.StatusCode >= 300
	}

	if err != nil || (responseReceived && badStatus) {
		var returnErr error
		if err == nil {
			returnErr = fmt.Errorf("export failed with %d HTTP status code in pipeline '%s'", response.StatusCode, ctx.PipelineId())
		} else {
			returnErr = fmt.Errorf("export failed in pipeline '%s': %v", ctx.PipelineId(), err.Error())
		}
		if response != nil {
			// print the error msg from the response
			body := response.Body
			defer body.Close()
			errResponseBytes, _ := io.ReadAll(body)
			var errResponse map[string]interface{}
			json.Unmarshal(errResponseBytes, &errResponse)
			lc.Errorf("Error returned: %v", errResponse)
			returnErr = fmt.Errorf("%s, error response: %s", returnErr.Error(), errResponse)
		}
		lc.Errorf("error making http call: %v", returnErr)
		lc.Debugf("Pipeline '%s': PersistOnError '%v'", ctx.PipelineId(), sender.PersistOnError)
		// http 400 is a bad request from client side, so no need to keep pumping to backend
		if sender.PersistOnError {
			if response != nil &&
				(response.StatusCode != http.StatusNotFound &&
					response.StatusCode != http.StatusBadRequest &&
					response.StatusCode != http.StatusConflict) {
				lc.Debugf("Pipeline '%s': Saving data", ctx.PipelineId())
				ctx.SetRetryData(exportData.storeForwardData)
			}
		}
		lc.Errorf("Export failed in pipeline '%s': %s", ctx.PipelineId(), returnErr.Error())
		lc.Errorf("Failed request '%s': %v", ctx.PipelineId(), req.URL)
		return false, response
	}

	lc.Debugf("Data exported for pipeline '%s' (%s=%s)", ctx.PipelineId(), common.CorrelationHeader, ctx.CorrelationID())

	defer response.Body.Close()
	lc.Debugf("Response data - %v", response)
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		lc.Errorf("body_read_error=%v", err)
		if sender.PersistOnError {
			ctx.SetRetryData(exportData.storeForwardData)
		}
		return false, err
	}

	return true, responseData
}

func (sender *HTTPSender) GetURL() string {
	return sender.URL
}

func (sender *HTTPSender) SetURL(url string) {
	sender.URL = url
}

func (sender *HTTPSender) GetMimeType() string {
	return sender.MimeType
}

func (sender *HTTPSender) SetPersistOnError(updatedPersistOnErrorValue bool) {
	sender.PersistOnError = updatedPersistOnErrorValue
}

func (sender *HTTPSender) GetPersistOnError() bool {
	return sender.PersistOnError
}
