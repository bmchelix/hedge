package utils

import (
	"hedge/common/models"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type HttpResponseData struct {
	Method     string
	Host       string //Optional if present, we match using Host as well
	Body       interface{}
	StatusCode int
	err        error
}

// MockClient is the mock client
type MockClient struct {
	// For mock GET calls, store the URL to body that we need to return in here, GetDoFunc will return the body based on matching URL
	Url2BodyMap map[string]HttpResponseData
}

// Do is the mock client's `Do` func
func (m MockClient) Do(req *http.Request) (*http.Response, error) {
	for url, responseData := range m.Url2BodyMap {

		if strings.HasPrefix(req.URL.Path, url) {
			// Now match the method
			if req.Method == responseData.Method {
				bodyBytes, err := json.Marshal(responseData.Body)
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: responseData.StatusCode,
					Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
				}, responseData.err
			}

		}
	}
	// Return 404 if not found
	return &http.Response{
		StatusCode: 404,
		Body:       nil,
	}, nil
	//return GetDoFunc(req)
}

var (
	//GetDoFunc func(req *http.Request) (*http.Response, error)
	Node1 models.Node
)

// Make sure that you assign this MockClient to global variable Client of type HTTPClient interface where we override the actual HTTPClient Do method
func NewMockClient() *MockClient {
	httpMockClient := new(MockClient)

	httpMockClient.Url2BodyMap = make(map[string]HttpResponseData)

	// Auto register node since this is used by many of our services to get the current Node during startup
	Node1 = models.Node{
		NodeId:           "nodeId01",
		HostName:         "clm-pun-xyz",
		IsRemoteHost:     false,
		Name:             "clm-pun-xyz",
		RuleEndPoint:     "http://edgex-kuiper:59887",
		WorkFlowEndPoint: "http://hedge-node-red:1880",
	}
	httpMockClient.Url2BodyMap["/api/v3/node_mgmt/node/current"] = HttpResponseData{
		Method:     "GET",
		Body:       Node1,
		StatusCode: 200,
		err:        nil,
	}

	// Do even the assignment of global var in here
	return httpMockClient
}

/*
Parameters are based on the order so we can provide variable parameters, other values are defaulted
parameters: url, body to be returned, httpstatus to be returned (default 200), error (default nil), method ( default is GET)
*/
func (m *MockClient) RegisterExternalMockRestCall(urlToMatch string, method string, responseData ...interface{}) {

	// Set the default body, then override if the parameters are provided
	httpResponseData := HttpResponseData{
		StatusCode: 200,
		Method:     method,
		Body:       nil,
		err:        nil,
	}

	for index, val := range responseData {
		switch index {
		case 0:
			httpResponseData.Body = val
		case 1:
			httpResponseData.StatusCode, _ = val.(int)
		case 2:
			httpResponseData.err, _ = val.(error)
		case 3:
			httpResponseData.Method, _ = val.(string)
		}
	}

	if strings.HasPrefix(urlToMatch, "http") {
		urlx, err := url.Parse(urlToMatch)
		if err == nil {
			httpResponseData.Host = urlx.Host
		}
		m.Url2BodyMap[urlx.Path] = httpResponseData
	} else {
		m.Url2BodyMap[urlToMatch] = httpResponseData
	}

}
