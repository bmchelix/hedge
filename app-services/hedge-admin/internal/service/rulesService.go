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
	"bytes"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"net/http"
	"os"
)

var (
	baseDir string
)

type RuleService struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
}

func NewRuleService(service interfaces.ApplicationService, appConfig *config.AppConfig) *RuleService {
	ruleService := new(RuleService)
	ruleService.service = service
	ruleService.appConfig = appConfig
	return ruleService
}

func (ruleService *RuleService) SetHttpClient() {
	if client.Client == nil {
		client.Client = &http.Client{}
	}
}

func (ruleService *RuleService) ImportStreamsAndRules(contentData models.ContentData) hedgeErrors.HedgeError {
	if contentData.TargetNodes == nil || len(contentData.TargetNodes) == 0 {
		contentData.TargetNodes = []string{"edgex-kuiper"}
	}
	err := ruleService.createStreams(contentData)
	if err != nil {
		ruleService.service.LoggingClient().Warn(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating streams")
	}
	err = ruleService.createRules(contentData)
	if err != nil {
		ruleService.service.LoggingClient().Warn(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating rules")
	}
	return nil
}

func (ruleService *RuleService) createStreams(contentData models.ContentData) hedgeErrors.HedgeError {
	streamsDir := ruleService.appConfig.ContentDir + "/" + "rules" + "/" + contentData.NodeType + "/" + "streams"
	err := ruleService.readDirectoryAndCreate(streamsDir, contentData, "streams")
	if err != nil {
		ruleService.service.LoggingClient().Warn(err.Error())
		return err
	}
	return nil
}

func (ruleService *RuleService) createRules(contentData models.ContentData) hedgeErrors.HedgeError {
	for _, directory := range contentData.ContentDir {
		rulesDir := ruleService.appConfig.ContentDir + "/" + "rules" + "/" + contentData.NodeType + "/" + directory + "/" + "rules"
		err := ruleService.readDirectoryAndCreate(rulesDir, contentData, "rules")
		if err != nil {
			ruleService.service.LoggingClient().Warn(err.Error())
			return err
		}
	}
	return nil
}

func (ruleService *RuleService) readDirectoryAndCreate(dir string, contentData models.ContentData, pathType string) hedgeErrors.HedgeError {

	files, err := os.ReadDir(dir)
	if err != nil {
		ruleService.service.LoggingClient().Warn(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error reading directory")
	}

	for _, file := range files {
		ruleService.service.LoggingClient().Info(fmt.Sprintf("Reading File: %s", file.Name()))
		fileName := dir + "/" + file.Name()
		err = ruleService.create(contentData, fileName, pathType)
		if err != nil {
			ruleService.service.LoggingClient().Warn(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating directory")
		}
	}
	return nil
}

func (ruleService *RuleService) create(contentData models.ContentData, fileName string, pathType string) hedgeErrors.HedgeError {

	for _, target := range contentData.TargetNodes {
		url := "http://" + target + ":59720" + "/" + pathType
		fmt.Println(url)
		byteData, err := os.ReadFile(fileName)
		if err != nil {
			ruleService.service.LoggingClient().Warn(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating Rule Service")
		}
		request, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(byteData))
		request.Header.Set("Content-type", "application/json")

		ruleService.SetHttpClient()
		resp, err := client.Client.Do(request)
		if err != nil {
			ruleService.service.LoggingClient().Warn(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating Rule Service")
		}

		err = resp.Body.Close()
		if err != nil {
			ruleService.service.LoggingClient().Error(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating Rule Service")
		}
	}

	return nil
}
