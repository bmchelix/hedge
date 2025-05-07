package connector

import (
	"hedge/app-services/hedge-device-extensions/internal/util"
	"hedge/common/dto"
	svcmocks "hedge/mocks/hedge/common/service"
	"bytes"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestGetEventCountByDevices(t *testing.T) {
	t.Run("GetEventCountByDevices - Passed", func(t *testing.T) {
		testVictoriaUrl := "http://mock-victoria"
		testDevices := []string{"device1", "device2"}
		testEdgeNode := "mock-edge"

		testResponsePayload := &dto.TimeSeriesResponse{
			Data: dto.TimeSeriesData{
				Result: []dto.MetricResult{
					{
						Metric: map[string]interface{}{"device": "device1"},
						Values: []interface{}{nil, "10"},
					},
					{
						Metric: map[string]interface{}{"device": "device2"},
						Values: []interface{}{nil, "20"},
					},
				},
			},
		}

		bodyBytes, _ := json.Marshal(testResponsePayload)
		mockedHttpClient := &svcmocks.MockHTTPClient{}
		testResponse := &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			Header:     make(http.Header),
		}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		result, err := GetEventCountByDevices(testVictoriaUrl, testDevices, testEdgeNode)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expected := map[string]int64{"device1": 10, "device2": 20}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected result: %v, but got: %v", expected, result)
		}
	})
	t.Run("GetEventCountByDevices - Failed (Failed to get response)", func(t *testing.T) {
		testVictoriaUrl := "http://mock-victoria"
		testDevices := []string{"device1"}
		testEdgeNode := "mock-edge"

		mockedHttpClient := &svcmocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New("dummy error"))
		util.HttpClient = mockedHttpClient

		result, err := GetEventCountByDevices(testVictoriaUrl, testDevices, testEdgeNode)
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
		if result != nil {
			t.Errorf("Expected result to be nil, but got: %v", result)
		}
	})
	t.Run("GetEventCountByDevices - Passed (Empty result)", func(t *testing.T) {
		testVictoriaUrl := "http://mock-victoria"
		var testDevices []string
		testEdgeNode := ""

		testResponsePayload := &dto.TimeSeriesResponse{
			Data: dto.TimeSeriesData{
				Result: []dto.MetricResult{},
			},
		}

		bodyBytes, _ := json.Marshal(testResponsePayload)
		mockedHttpClient := &svcmocks.MockHTTPClient{}
		testResponse := &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			Header:     make(http.Header),
		}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		result, err := GetEventCountByDevices(testVictoriaUrl, testDevices, testEdgeNode)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		expected := map[string]int64{}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected result: %v, but got: %v", expected, result)
		}
	})
}

func TestBuildEventQuery(t *testing.T) {
	t.Run("buildEventQuery - Passed (With devices and edge node)", func(t *testing.T) {
		devices := []string{"device1", "device2"}
		edgeNode := "edgeNode1"
		expected := "sum by(device) (IoTEvent{edgeNode=\"edgeNode1\",device=~\"device1|device2\"}[2w])"

		result := buildEventQuery(devices, edgeNode)

		if result != expected {
			t.Errorf("Expected query: %s, but got: %s", expected, result)
		}
	})
	t.Run("buildEventQuery - Passed (With devices only)", func(t *testing.T) {
		devices := []string{"device1", "device2"}
		edgeNode := ""
		expected := "sum by(device) (IoTEvent{device=~\"device1|device2\"}[2w])"

		result := buildEventQuery(devices, edgeNode)

		if result != expected {
			t.Errorf("Expected query: %s, but got: %s", expected, result)
		}
	})
	t.Run("buildEventQuery - Passed (With edge node only)", func(t *testing.T) {
		devices := []string{}
		edgeNode := "edgeNode1"
		expected := "sum by(device) (IoTEvent{edgeNode=\"edgeNode1\",}[2w])"

		result := buildEventQuery(devices, edgeNode)

		if result != expected {
			t.Errorf("Expected query: %s, but got: %s", expected, result)
		}
	})
	t.Run("buildEventQuery - Passed (No devices or edge node)", func(t *testing.T) {
		devices := []string{}
		edgeNode := ""
		expected := "sum by(device) (IoTEvent{}[2w])"

		result := buildEventQuery(devices, edgeNode)

		if result != expected {
			t.Errorf("Expected query: %s, but got: %s", expected, result)
		}
	})
}
