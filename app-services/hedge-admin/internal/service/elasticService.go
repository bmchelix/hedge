/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-admin/internal/config"
	"hedge/app-services/hedge-admin/models"
	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	AuthorizationHeader = "Basic YWRtaW46YWRtaW4="
)

type ElasticService struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
}

type deviceMap struct {
	Id         string            `json:"id" codec:"id"`
	DeviceName string            `json:"deviceName" codec:"deviceName"`
	Profile    string            `json:"profile" codec:"profile"`
	Region     string            `json:"region" codec:"region"`
	Latitude   float64           `json:"latitude" codec:"latitude"`
	Longitude  float64           `json:"longitude" codec:"longitude"`
	Custom1    map[string]string `json:"custom1" codec:"custom1"`
	Custom2    map[string]string `json:"custom2" codec:"custom2"`
	Custom3    map[string]string `json:"custom3" codec:"custom3"`
	Custom4    map[string]string `json:"custom4" codec:"custom4"`
	Custom5    map[string]string `json:"custom5" codec:"custom5"`
	Created    int64             `json:"created" codec:"created"`
}

func NewElasticService(service interfaces.ApplicationService, appConfig *config.AppConfig) *ElasticService {
	return &ElasticService{
		service:   service,
		appConfig: appConfig,
	}
}

func (elasticService *ElasticService) SetHttpClient() {
	if client.Client == nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Client = &http.Client{Transport: tr}
	}
}

func (elasticService *ElasticService) SeedElasticData(contentData models.ContentData) error {

	if contentData.TargetNodes == nil || len(contentData.TargetNodes) == 0 {
		contentData.TargetNodes = []string{"hedge-elasticsearch"}
	}
	err := elasticService.seedElasticData(contentData)
	if err != nil {
		elasticService.service.LoggingClient().Warn(err.Error())
		return err
	}
	return nil
}

func (elasticService *ElasticService) seedElasticData(contentData models.ContentData) hedgeErrors.HedgeError {
	lc := elasticService.service.LoggingClient()
	seedMapDir := elasticService.appConfig.ContentDir + "/" + "elasticsearch/demo-seed-data/"

	errorMessage := "Error seeding Elastic data"

	var err error
	files, err := os.ReadDir(seedMapDir)
	if err != nil {
		lc.Errorf("Error reading the directory %s: %s", seedMapDir, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	for _, file := range files {
		lc.Infof("Reading File: %s", file.Name())
		if !strings.HasSuffix(file.Name(), ".json") {
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: File should be of json type", errorMessage))
		}
		jsonBytes, err := os.ReadFile(seedMapDir + file.Name())
		if err != nil {
			lc.Errorf("Error reading the file: %s", err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: Error Reading File: %s", errorMessage, file.Name()))
		}

		var jsonArr []deviceMap
		err = json.Unmarshal(jsonBytes, &jsonArr)
		if err != nil {
			lc.Errorf("Failed unmarshalling the json content: %s", err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
		// payload := strings.NewReader(`{"id":"1016","latitude":37.751942,"longitude":	-122.502514,"region":	1,		"profile": "Telco-Tower" ,"deviceName" : "San Francisco.SF Battery Dynamite","created":1614326098614} ` + "" + ` `)

		lc.Infof("Pumping %d ES entries from %s.", len(jsonArr), file.Name())
		for j, jsonObj := range jsonArr {
			lc.Debugf("Creating %s #%d id=%s", file.Name(), j, jsonObj.Id)
			//jsonStr := fmt.Sprintf("%v", jsonObj)

			if jsonObj.Created == 0 {
				jsonObj.Created = time.Now().UnixNano() / 1000000
			}

			jsonByte, err := json.Marshal(jsonObj)
			if err != nil {
				lc.Errorf("Failed marshalling a single demo json. Error: %v", err)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}
			payload := strings.NewReader(string(jsonByte))
			for _, target := range contentData.TargetNodes {
				url := "http://" + target + ":9200/devicemap_index/_doc/" + jsonObj.Id + "?refresh"
				method := "PUT"

				req, err := http.NewRequest(method, url, payload)

				if err != nil {
					lc.Errorf("Error creating request: %v", err)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
				}
				req.Header.Add("Authorization", AuthorizationHeader)
				req.Header.Add("Content-Type", "application/json")

				elasticService.SetHttpClient()
				res, reqErr := client.Client.Do(req)
				if reqErr != nil {
					lc.Errorf("Error sending the HTTP Request: %v", reqErr)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
				}

				body, _ := io.ReadAll(res.Body)

				// Error out unless - 200 success or 409 conflict
				if res.StatusCode != 200 && res.StatusCode != 201 && res.StatusCode != 409 {
					lc.Errorf("Failed creation. Error: %s", res.Status)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
				}

				lc.Debugf("Response: %s", string(body))

				err = res.Body.Close()
				if err != nil {
					lc.Errorf("Error closing response body stream %v", err)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
				}
			}
		}

		//sleep for 2 sec before pushing the next item
		time.Sleep(2 * time.Second)
		lc.Infof("====== Successfully created %d %s items =======", len(jsonArr), file.Name())
	}

	return nil
}
