package service

import (
	"archive/zip"
	"hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	svcmocks "hedge/mocks/hedge/common/service"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const baseTmpFolder = "tmp"

func InitDataMgmtServiceTest() DataMgmtService {
	dataMgmtService := DataMgmtService{
		service:   mockUtils.AppService,
		appConfig: appConfig,
	}
	return dataMgmtService
}

func CleanupFolder(folderPath string, t *testing.T) {
	_, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		return
	}
	// Attempt to delete the folder and its content
	if err := os.RemoveAll(folderPath); err != nil {
		t.Errorf("Failed to delete file: %v", err)
	}
}

func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		// If the error is not because the file doesn't exist, log it
		if !os.IsNotExist(err) {
			fmt.Printf("Error checking file: %v\n", err)
			return false
		}
		return false
	}

	// Check if it is indeed a file and not a directory
	return !info.IsDir()
}

func createTestZipFile(fileName string, fileContent []byte) (*os.File, error) {
	_ = os.MkdirAll(filepath.Dir(fileName), os.ModePerm)
	file, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	w, err := zipWriter.Create("data.json")
	if err != nil {
		return nil, err
	}
	_, err = w.Write(fileContent)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func createTestZipFileForParseZipFile(indexData interface{}, flowsData []dto.Flow, rulesData []dto.Rule, profilesData []dto.ProfileObject) (*bytes.Buffer, error) {
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	indexFileContent, err := json.Marshal(indexData)
	if err != nil {
		return nil, err
	}
	indexFile, err := zipWriter.Create("index.json")
	if err != nil {
		return nil, err
	}
	_, err = indexFile.Write(indexFileContent)
	if err != nil {
		return nil, err
	}

	flowsFileContent, err := json.Marshal(flowsData)
	if err != nil {
		return nil, err
	}
	flowsFile, err := zipWriter.Create("flows.json")
	if err != nil {
		return nil, err
	}
	_, err = flowsFile.Write(flowsFileContent)
	if err != nil {
		return nil, err
	}

	rulesFileContent, err := json.Marshal(rulesData)
	if err != nil {
		return nil, err
	}
	rulesFile, err := zipWriter.Create("rules.json")
	if err != nil {
		return nil, err
	}
	_, err = rulesFile.Write(rulesFileContent)
	if err != nil {
		return nil, err
	}

	profilesFileContent, err := json.Marshal(profilesData)
	if err != nil {
		return nil, err
	}
	profilesFile, err := zipWriter.Create("profiles.json")
	if err != nil {
		return nil, err
	}
	_, err = profilesFile.Write(profilesFileContent)
	if err != nil {
		return nil, err
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	return &zipBuffer, nil
}

func TestDataMgmtService_NewDataMgmtService(t *testing.T) {
	InitDataMgmtServiceTest()
	dataMgmtService := NewDataMgmtService(mockUtils.AppService, appConfig)

	assert.NotNil(t, dataMgmtService)
	assert.Equal(t, mockUtils.AppService, dataMgmtService.service)
	assert.Equal(t, appConfig, dataMgmtService.appConfig)
}

func TestDataMgmtService_SetHttpClient(t *testing.T) {
	client.Client = nil
	setHttpClient()

	assert.NotNil(t, client.Client)
	assert.IsType(t, &http.Client{}, client.Client)

	existingClient := client.Client
	setHttpClient()

	assert.Equal(t, existingClient, client.Client)
}

func TestDataMgmtService_getCompleteProfile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()
	profileName := "test-profile"
	expectedProfile := dto.ProfileObject{Profile: dtos.DeviceProfile{
		DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
			Name: profileName,
		},
	}}
	jsonData, _ := json.Marshal(expectedProfile)

	setup1 := func() {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(jsonData)),
		}
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		mockedHttpClient1.On("Do", mock.Anything).Return(resp, nil)
		client.Client = mockedHttpClient1
	}
	setup2 := func() {
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		mockedHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
		client.Client = mockedHttpClient1
	}
	setup3 := func() {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
		}
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		mockedHttpClient1.On("Do", mock.Anything).Return(resp, nil)
		client.Client = mockedHttpClient1
	}

	tests := []struct {
		name           string
		setup          func()
		expectedError  bool
		expectedLength int
	}{
		{"GetCompleteProfile - Passed", setup1, false, 1},
		{"GetCompleteProfile - Failed", setup2, true, 0},
		{"GetCompleteProfile - Failed JSON Decoding", setup3, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			profiles := []dto.ProfileObject{}
			err := getCompleteProfile(profileName, &expectedProfile, &profiles, dataMgmtService)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "Error fetching complete profile")
			} else {
				assert.Nil(t, err)
			}
			assert.Len(t, profiles, tt.expectedLength)
			if tt.expectedLength > 0 {
				assert.Equal(t, expectedProfile, profiles[0])
			}
		})
	}
}

func TestDataMgmtService_checkZipFileSize_SmallFile_Pass(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	filePath := filepath.Join(baseTmpFolder, "test.zip")
	_ = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipFile, _ := os.Create(filePath)
	_, _ = zipFile.WriteString("small content")
	defer zipFile.Close()

	err := dataMgmtService.checkZipFileSize(zipFile)

	if err != nil {
		t.Errorf("checkZipFileSize returned error, but expected success: %v", err)
	}
}

func TestDataMgmtService_checkZipFileSize_LargeFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	filePath := filepath.Join(baseTmpFolder, "test.zip")
	_ = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipFile, _ := os.Create(filePath)
	_, _ = zipFile.Write(make([]byte, appConfig.MaxImportExportFileSizeMB*1024*1024+1))
	defer zipFile.Close()

	err := dataMgmtService.checkZipFileSize(zipFile)

	if err == nil {
		t.Errorf("checkZipFileSize returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "File size exceeds the allowed limit")
	if !reflect.DeepEqual(err, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", err, expectedError)
	}
}

func TestDataMgmtService_checkZipFileSize_FileDoesntExist_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	err := dataMgmtService.checkZipFileSize(nil)

	if err == nil {
		t.Errorf("checkZipFileSize returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Could not get file info")
	if !reflect.DeepEqual(err, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", err, expectedError)
	}
}

func TestDataMgmtService_checkZipFile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	tests := []struct {
		name          string
		fileName      string
		expectedError bool
		expectedMsg   string
	}{
		{"CheckZipFile - Passed", "testfile.zip", false, ""},
		{"CheckZipFile - Failed Invalid Zip File", "testfile.txt", true, "File is not in '.zip' format"},
		{"CheckZipFile - Failed No Extension File", "testfile", true, "File is not in '.zip' format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckZipFile(dataMgmtService, tt.fileName)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "File should be in zip format")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDataMgmtService_checkJsonFile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	tests := []struct {
		name            string
		fileName        string
		expectedError   error
		expectedMessage string
	}{
		{"CheckJsonFile - Passed", "valid-file.json", nil, "Expected no error for a valid JSON file"},
		{"CheckJsonFile - Failed Not a JSON File", "invalid-file.txt", errors.New("File should be in json format"), "Expected an error for a file that is not in JSON format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hedgeErr := checkJsonFile(dataMgmtService, tt.fileName)

			if tt.expectedError != nil {
				assert.NotNil(t, hedgeErr)
				assert.Contains(t, hedgeErr.Error(), tt.expectedError.Error(), tt.expectedMessage)
			} else {
				assert.Nil(t, hedgeErr, tt.expectedMessage)
			}
		})
	}
}

func TestDataMgmtService_getRuleList(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	expectedBodyMap := []map[string]interface{}{
		{"rule": "rule1", "status": "active"},
		{"rule": "rule2", "status": "inactive"},
	}
	jsonData, _ := json.Marshal(expectedBodyMap)
	response1 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}
	response2 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}
	response3 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
	}
	tests := []struct {
		name            string
		node            dto.ExportNodeData
		response        *http.Response
		expectedError   error
		expectedBodyMap []map[string]interface{}
	}{
		{"GetRuleList - Local Node - Passed", dto.ExportNodeData{NodeID: dataMgmtService.appConfig.NodeId}, response1, nil, expectedBodyMap},
		{"GetRuleList - Remote Node - Passed", dto.ExportNodeData{NodeID: "remote-node-id"}, response2, nil, expectedBodyMap},
		{"GetRuleList - Failed Request", dto.ExportNodeData{NodeID: "remote-node-id"}, nil, errors.New("Error fetching rule list from node remote-node-id"), []map[string]interface{}{}},
		{"GetRuleList - Failed JSON Decoding", dto.ExportNodeData{NodeID: dataMgmtService.appConfig.NodeId}, response3, errors.New("Error fetching rule list from node"), []map[string]interface{}{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyMap := []map[string]interface{}{}
			mockedHttpClient1 := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient1.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient1

			err := getRuleList(dataMgmtService, tt.node, &bodyMap)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Equal(t, tt.expectedBodyMap, bodyMap)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.expectedBodyMap, bodyMap)
			}
		})
	}
}

func TestDataMgmtService_createRuleData(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	ruleId := "test-rule-id"
	nodeLocal := dto.ExportNodeData{NodeID: dataMgmtService.appConfig.NodeId}
	nodeRemote := dto.ExportNodeData{NodeID: "remote-node-id"}

	bodyMap := []map[string]interface{}{
		{"id": ruleId, "name": "Test Rule"},
	}
	expectedRuleData := dto.RuleData{Id: ruleId, Sql: "Test Rule Sql Data"}
	jsonData, _ := json.Marshal(expectedRuleData)
	response1 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}
	response2 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}
	response3 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
	}
	tests := []struct {
		name             string
		node             dto.ExportNodeData
		response         *http.Response
		expectedError    error
		expectedRuleData *dto.RuleData
	}{
		{"CreateRuleData - Local Node - Passed", nodeLocal, response1, nil, &expectedRuleData},
		{"CreateRuleData - Remote Node - Passed", nodeRemote, response2, nil, &expectedRuleData},
		{"CreateRuleData - Failed Request", nodeRemote, nil, errors.New(fmt.Sprintf("Error creating rule id %s data for node %s", ruleId, nodeRemote.NodeID)), &dto.RuleData{}},
		{"CreateRuleData - Failed JSON Decoding", nodeLocal, response3, errors.New(fmt.Sprintf("Error creating rule id %s data for node ", ruleId)), &dto.RuleData{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient1 := &svcmocks.MockHTTPClient{}
			ruleData := &dto.RuleData{}

			if tt.response != nil {
				mockedHttpClient1.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient1

			err := createRuleData(dataMgmtService, tt.node, bodyMap, ruleData, ruleId)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Equal(t, tt.expectedRuleData, ruleData)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, ruleData, "Expected ruleData to be populated")
				assert.Equal(t, tt.expectedRuleData, ruleData)
			}
		})
	}
}

func TestDataMgmtService_exportProfiles(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()
	exportImportDataMock := dto.ExportImportData{
		ProfilesData: dto.ExportImportProfiles{
			Profiles: []dto.ProfileObject{{Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}}}},
			FilePath: "/mock/path/profiles.json",
			ProfilesIndexData: []string{
				"Profile1",
				"Profile2",
			},
			Errors: []dto.ProfileError{},
		},
	}

	profileName := "test-profile"
	profileNames := []string{profileName}
	expectedProfile := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	jsonData, _ := json.Marshal(expectedProfile)

	setup1 := func() {
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(jsonData)),
		}
		mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet &&
				req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
		})).Return(resp, nil)
		client.Client = mockedHttpClient1
	}

	setup2 := func() {
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet &&
				req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
		})).Return(nil, errors.New("request failed"))
		client.Client = mockedHttpClient1
	}

	tests := []struct {
		name          string
		setup         func()
		expectedError error
	}{
		{"ExportProfiles - Passed", setup1, nil},
		{"ExportProfiles - Failed", setup2, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error fetching complete profile "+profileName)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			indexFile, err := os.CreateTemp("", "index-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp index file: %v", err)
			}
			defer os.Remove(indexFile.Name())

			err = dataMgmtService.exportProfiles(profileNames, &exportImportDataMock.ProfilesData)

			if tt.expectedError != nil || len(exportImportDataMock.ProfilesData.Errors) > 0 {
				assert.Nil(t, err, "Expected no top-level error since errors are tracked in ProfilesData.Errors")
				assert.NotEmpty(t, exportImportDataMock.ProfilesData.Errors, "Expected errors in ProfilesData.Errors")
				assert.Contains(t, exportImportDataMock.ProfilesData.Errors[0].ProfileID, profileName)
				t.Logf("Idos error %v", exportImportDataMock.ProfilesData.Errors[0].Errors[0])
				assert.EqualError(t, exportImportDataMock.ProfilesData.Errors[0].Errors[0], tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
				assert.Equal(t, exportImportDataMock.ProfilesData.ProfilesIndexData, profileNames)
				assert.Equal(t, exportImportDataMock.ProfilesData.Profiles[0], expectedProfile)
				assert.Empty(t, exportImportDataMock.ProfilesData.Errors, "Expected no errors in ProfilesData.Errors")
			}
		})
	}
}

func TestDataMgmtService_removeJsonFiles_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath1 := filepath.Join(baseTmpFolder, "flows.json")
	filePath2 := filepath.Join(baseTmpFolder, "rules.json")
	file1, _ := os.Create(filePath1)
	file2, _ := os.Create(filePath2)

	file1.Close()
	file2.Close()

	filePaths := []string{filePath1, filePath2}

	hedgeError := dataMgmtService.removeJsonFiles(filePaths)

	if hedgeError != nil {
		t.Errorf("removeJsonFiles returned error, but expected success %v", hedgeError)
	}

	for _, filePath := range filePaths {
		if FileExists(filePath) {
			t.Errorf("`%s` is found but expected to be removed", filePath)
		}
	}
}

func TestDataMgmtService_removeJsonFiles_InvalidPath_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePaths := []string{"invalid/path"}

	hedgeError := dataMgmtService.removeJsonFiles(filePaths)

	if hedgeError != nil {
		t.Errorf("removeJsonFiles returned error, but expected success %v", hedgeError)
	}

	for _, filePath := range filePaths {
		if FileExists(filePath) {
			t.Errorf("`%s` is found but expected to be removed", filePath)
		}
	}

}

func TestDataMgmtService_removeJsonFiles_SomeDeletedSomeNot_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath1 := filepath.Join(baseTmpFolder, "flows.json")
	file1, _ := os.Create(filePath1)

	file1.Close()

	filePaths := []string{filePath1, "invalid/path"}

	hedgeError := dataMgmtService.removeJsonFiles(filePaths)

	if hedgeError != nil {
		t.Errorf("removeJsonFiles returned error, but expected success %v", hedgeError)
	}

	if FileExists(filePath1) {
		t.Errorf("`%s` is found but expected to be removed", filePath1)
	}
}

func TestDataMgmtService_removeZipFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipPath := filepath.Join(baseTmpFolder, "test.zip")

	file, _ := os.Create(zipPath)
	file.Close()

	hedgeError := dataMgmtService.removeZipFile(zipPath)

	if hedgeError != nil {
		t.Errorf("removeJsonFiles returned error, but expected success %v", hedgeError)
	}

	if FileExists(zipPath) {
		t.Errorf("`%s` is found but expected to be removed", zipPath)
	}
}

func TestDataMgmtService_removeZipFile_FileNotFound_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	hedgeError := dataMgmtService.removeZipFile(filepath.Join(baseTmpFolder, "test.zip"))

	if hedgeError != nil {
		t.Errorf("removeJsonFiles returned error, but expected success %v", hedgeError)
	}
}

func TestDataMgmtService_StreamZipFile_Pass(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set("Content-Type", "application/octet-stream")

	filePath := filepath.Join(baseTmpFolder, "test.zip")
	_ = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	file, _ := os.Create(filePath)
	_ = file.Close()

	hedgeErr := dataMgmtService.StreamZipFile(c, filePath)

	if hedgeErr != nil {
		t.Errorf("StreamZipFile returned error, but expected success: %v", hedgeErr)
	}

	_, err := os.Stat(filePath)
	if !os.IsNotExist(err) {
		t.Errorf("Zip file should've been removed, but is still found on the disk %s", filePath)
	}
}

func TestDataMgmtService_StreamZipFile_ExactLimitFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set("Content-Type", "application/octet-stream")

	filePath := filepath.Join(baseTmpFolder, "test.zip")
	_ = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	file, _ := os.Create(filePath)
	_, _ = file.Write(make([]byte, appConfig.MaxImportExportFileSizeMB*1024*1024*2)) // Exactly 1 MB
	_ = file.Close()

	hedgeErr := dataMgmtService.StreamZipFile(c, filePath)

	if hedgeErr == nil {
		t.Errorf("StreamZipFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "File size exceeds the allowed limit")
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeErr, expectedError)
	}

	_, err := os.Stat(filePath)
	if !os.IsNotExist(err) {
		t.Errorf("Zip file should've been removed, but is still found on the disk %s", filePath)
	}
}

func TestDataMgmtService_streamZipFile_FileDoesntExist_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set("Content-Type", "application/octet-stream")

	filePath := filepath.Join(baseTmpFolder, "test.zip")

	hedgeErr := dataMgmtService.StreamZipFile(c, filePath)

	if hedgeErr == nil {
		t.Errorf("streamZipFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error streaming zip file")
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeErr, expectedError)
	}
}

func TestDataMgmtService_createIndexFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportData := dto.ExportImportData{
		IndexData: dto.ExportImportIndex{
			Imports: dto.NodeData{
				ToNodeId: []string{"Node1", "Node2"},
				Flows:    []string{"Flow1", "Flow2"},
				Rules:    []string{"Rule1", "Rule2"},
				Profiles: []string{"Profile1", "Profile2"},
			},
			FilePath: "/mock/path/index.json",
		},
		FlowsData: dto.ExportImportFlow{
			Flows: []dto.Flow{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Selected:   true,
					Data: map[string]interface{}{
						"id":    "flow1",
						"label": "Test Flow",
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Selected:   false,
					Data: map[string]interface{}{
						"key": "flow2",
					},
				},
			},
			FilePath: "/mock/path/flows.json",
			FlowsIndexData: []map[string]interface{}{
				{
					"flowId": "Flow1",
					"label":  "Test Flow",
				},
				{
					"flowId": "Flow2",
					"status": "Test Flow",
				},
			},
			Errors: nil,
		},
		RulesData: dto.ExportImportRules{
			Rules: []dto.Rule{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Name:       []string{"Rule1"},
					Data: dto.RuleData{
						Id:  "rule-001",
						Sql: "SELECT * FROM data WHERE condition = true",
						Actions: []interface{}{
							"Action1",
							"Action2",
						},
						Options: map[string]interface{}{
							"option1": true,
							"option2": "value",
						},
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Name:       []string{"Rule2"},
					Data: dto.RuleData{
						Id:  "rule-002",
						Sql: "SELECT * FROM data WHERE condition = false",
						Actions: []interface{}{
							"Action3",
						},
						Options: nil,
					},
				},
			},
			FilePath: "/mock/path/rules.json",
			RulesIndexData: []map[string]interface{}{
				{
					"ruleId": "Rule1",
					"status": "active",
				},
				{
					"ruleId": "Rule2",
					"status": "inactive",
				},
			},
			Errors: nil,
		},
		ProfilesData: dto.ExportImportProfiles{
			Profiles: []dto.ProfileObject{{Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}}}},
			FilePath: "/mock/path/profiles.json",
			ProfilesIndexData: []string{
				"Profile1",
				"Profile2",
			},
			Errors: nil,
		},
	}

	baseDir := filepath.Join(baseTmpFolder)

	_ = os.MkdirAll(baseDir, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	hedgeErr := dataMgmtService.createIndexFile(baseDir, &exportImportData)

	if hedgeErr != nil {
		t.Errorf("createIndexFile returned error, but expected success: %v", hedgeErr)
	}

	expectedFilePath := filepath.Join(baseTmpFolder, INDEX_FILE_NAME)
	if !FileExists(expectedFilePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", expectedFilePath)
	}
}

func TestDataMgmtService_createIndexFile_NoFolder_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportData := dto.ExportImportData{
		IndexData: dto.ExportImportIndex{
			Imports: dto.NodeData{
				ToNodeId: []string{"Node1", "Node2"},
				Flows:    []string{"Flow1", "Flow2"},
				Rules:    []string{"Rule1", "Rule2"},
				Profiles: []string{"Profile1", "Profile2"},
			},
			FilePath: "/mock/path/index.json",
		},
		FlowsData: dto.ExportImportFlow{
			Flows: []dto.Flow{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Selected:   true,
					Data: map[string]interface{}{
						"id":    "flow1",
						"label": "Test Flow",
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Selected:   false,
					Data: map[string]interface{}{
						"key": "flow2",
					},
				},
			},
			FilePath: "/mock/path/flows.json",
			FlowsIndexData: []map[string]interface{}{
				{
					"flowId": "Flow1",
					"label":  "Test Flow",
				},
				{
					"flowId": "Flow2",
					"status": "Test Flow",
				},
			},
			Errors: nil,
		},
		RulesData: dto.ExportImportRules{
			Rules: []dto.Rule{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Name:       []string{"Rule1"},
					Data: dto.RuleData{
						Id:  "rule-001",
						Sql: "SELECT * FROM data WHERE condition = true",
						Actions: []interface{}{
							"Action1",
							"Action2",
						},
						Options: map[string]interface{}{
							"option1": true,
							"option2": "value",
						},
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Name:       []string{"Rule2"},
					Data: dto.RuleData{
						Id:  "rule-002",
						Sql: "SELECT * FROM data WHERE condition = false",
						Actions: []interface{}{
							"Action3",
						},
						Options: nil,
					},
				},
			},
			FilePath: "/mock/path/rules.json",
			RulesIndexData: []map[string]interface{}{
				{
					"ruleId": "Rule1",
					"status": "active",
				},
				{
					"ruleId": "Rule2",
					"status": "inactive",
				},
			},
			Errors: nil,
		},
		ProfilesData: dto.ExportImportProfiles{
			Profiles: []dto.ProfileObject{{Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}}}},
			FilePath: "/mock/path/profiles.json",
			ProfilesIndexData: []string{
				"Profile1",
				"Profile2",
			},
			Errors: nil,
		},
	}

	hedgeErr := dataMgmtService.createIndexFile(baseTmpFolder, &exportImportData)

	if hedgeErr == nil {
		t.Errorf("createIndexFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error creating index file")
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeErr, expectedError)
	}

	expectedFilePath := filepath.Join(baseTmpFolder, INDEX_FILE_NAME)
	if FileExists(expectedFilePath) {
		t.Errorf("Expected file %s not to exist, but it is was found", expectedFilePath)
	}
}

func TestDataMgmtService_createIndexFile_NilExportData_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	hedgeErr := dataMgmtService.createIndexFile(baseTmpFolder, nil)

	if hedgeErr == nil {
		t.Errorf("createIndexFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed to create index file")
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeErr, expectedError)
	}

	expectedFilePath := filepath.Join(baseTmpFolder, INDEX_FILE_NAME)
	if FileExists(expectedFilePath) {
		t.Errorf("Expected file %s not to exist, but it is was found", expectedFilePath)
	}
}

func TestDataMgmtService_createZipFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	exportData := dto.ExportImportData{
		FlowsData:    dto.ExportImportFlow{FilePath: filepath.Join(baseTmpFolder, "flows.json")},
		RulesData:    dto.ExportImportRules{FilePath: filepath.Join(baseTmpFolder, "rules.json")},
		ProfilesData: dto.ExportImportProfiles{FilePath: filepath.Join(baseTmpFolder, "profiles.json")},
		IndexData:    dto.ExportImportIndex{FilePath: filepath.Join(baseTmpFolder, "index.json")},
	}

	filePaths := []string{
		exportData.FlowsData.FilePath,
		exportData.RulesData.FilePath,
		exportData.ProfilesData.FilePath,
		exportData.IndexData.FilePath,
	}

	for _, filePath := range filePaths {
		file, _ := os.Create(filePath)
		file.Close()
	}

	zipPath, hedgeError := dataMgmtService.createZipFile(baseTmpFolder, exportData)

	if hedgeError != nil {
		t.Errorf("createZipFile returned error, but expected success %v", hedgeError)
	}
	_, statErr := os.Stat(zipPath)
	assert.Contains(t, zipPath, filepath.Join(baseTmpFolder, EXPORT_DATA_FILE_NAME))
	assert.False(t, os.IsNotExist(statErr), fmt.Sprintf("Expected the zip file %s to exist", zipPath))

	for _, filePath := range filePaths {
		_, statErr = os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), fmt.Sprintf("Expected file %s to be deleted after zipping", filePath))
	}
}

func TestDataMgmtService_createZipFile_FileNotFound_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	exportData := dto.ExportImportData{
		FlowsData:    dto.ExportImportFlow{FilePath: filepath.Join(baseTmpFolder, "flows.json")},
		RulesData:    dto.ExportImportRules{FilePath: filepath.Join(baseTmpFolder, "rules.json")},
		ProfilesData: dto.ExportImportProfiles{FilePath: filepath.Join(baseTmpFolder, "profiles.json")},
		IndexData:    dto.ExportImportIndex{FilePath: filepath.Join(baseTmpFolder, "index.json")},
	}

	filePaths := []string{
		exportData.RulesData.FilePath,
		exportData.ProfilesData.FilePath,
		exportData.IndexData.FilePath,
	}

	for _, filePath := range filePaths {
		file, _ := os.Create(filePath)
		file.Close()
	}

	_, hedgeError := dataMgmtService.createZipFile(baseTmpFolder, exportData)

	if hedgeError == nil {
		t.Errorf("createZipFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Error creating zip file: file not found")
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}

	for _, filePath := range filePaths {
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), fmt.Sprintf("Expected file %s to be deleted after zipping", filePath))
	}
}

func TestDataMgmtService_createZipFile_ErrorCreatingZip_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportData := dto.ExportImportData{
		FlowsData:    dto.ExportImportFlow{FilePath: filepath.Join(baseTmpFolder, "flows.json")},
		RulesData:    dto.ExportImportRules{FilePath: filepath.Join(baseTmpFolder, "rules.json")},
		ProfilesData: dto.ExportImportProfiles{FilePath: filepath.Join(baseTmpFolder, "profiles.json")},
		IndexData:    dto.ExportImportIndex{FilePath: filepath.Join(baseTmpFolder, "index.json")},
	}

	filePaths := []string{
		exportData.FlowsData.FilePath,
		exportData.RulesData.FilePath,
		exportData.ProfilesData.FilePath,
		exportData.IndexData.FilePath,
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	for _, filePath := range filePaths {
		file, _ := os.Create(filePath)
		file.Close()
	}

	_, hedgeError := dataMgmtService.createZipFile("nonexistent", exportData)

	if hedgeError == nil {
		t.Errorf("createZipFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error creating zip file")
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}

	for _, filePath := range filePaths {
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), fmt.Sprintf("Expected file %s to be deleted after zipping", filePath))
	}
}

func TestDataMgmtService_createZipFile_NoExportData_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportData := dto.ExportImportData{}

	_, hedgeError := dataMgmtService.createZipFile(baseTmpFolder, exportData)

	if hedgeError == nil {
		t.Errorf("createZipFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "There is no data to export")
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_exportFlows(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportDataMock := dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			Flows: []dto.Flow{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Selected:   true,
					Data: map[string]interface{}{
						"id":    "flow1",
						"label": "Test Flow",
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Selected:   false,
					Data: map[string]interface{}{
						"id": "flow2",
					},
				},
			},
			FilePath: "/mock/path/flows.json",
			FlowsIndexData: []map[string]interface{}{
				{
					"flowId": "Flow1",
					"status": "active",
				},
				{
					"flowId": "Flow2",
					"status": "inactive",
				},
			},
			Errors: nil,
		},
	}

	nodes := []dto.ExportNodeData{
		{
			NodeID: dataMgmtService.appConfig.NodeId,
			Ids:    []string{"flow1"},
		},
		{
			NodeID: "remote-node-id",
			Ids:    []string{"flow3"},
		},
		{
			NodeID: "remote-node-id1",
			Ids:    nil,
		},
	}

	flowData := map[string]interface{}{
		"id":    "flow1",
		"label": "Test Flow",
	}
	flowJsonData, _ := json.Marshal(flowData)

	response1 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}
	response2 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	tests := []struct {
		name          string
		node          dto.ExportNodeData
		response      *http.Response
		expectedError error
	}{
		{"ExportFlows - Local Node - Passed", nodes[0], response1, nil},
		{"ExportFlows - Remote Node - Passed", nodes[1], response2, nil},
		{"ExportFlows - Failed Empty node ids list", nodes[2], nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "The node should contain a list of workflow ids")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			err := dataMgmtService.exportFlows([]dto.ExportNodeData{tt.node}, &exportImportDataMock.FlowsData)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
				assert.NotEmpty(t, exportImportDataMock.FlowsData.Flows, "Expected flows to be exported")
				assert.Equal(t, exportImportDataMock.FlowsData.Flows[0].Data, flowData)
			}
		})
	}
}

func TestDataMgmtService_exportFlowPerNodeId(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportDataMock := dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			Flows: []dto.Flow{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Selected:   true,
					Data: map[string]interface{}{
						"id":    "flow1",
						"label": "Test Flow",
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Selected:   false,
					Data: map[string]interface{}{
						"key": "flow2",
					},
				},
			},
			FilePath: "/mock/path/flows.json",
			FlowsIndexData: []map[string]interface{}{
				{
					"flowId": "Flow1",
					"status": "active",
				},
				{
					"flowId": "Flow2",
					"status": "inactive",
				},
			},
			Errors: nil,
		},
	}

	flowData := map[string]interface{}{
		"id":    "flow1",
		"label": "Test Flow",
	}
	flowJsonData, _ := json.Marshal(flowData)

	responseLocalNode := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	responseRemoteNode := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	tests := []struct {
		name          string
		nodeId        string
		flowId        string
		response      *http.Response
		expectedError error
	}{
		{"ExportFlowPerNodeId - Local Node - Passed", dataMgmtService.appConfig.NodeId, "flow1", responseLocalNode, nil},
		{"ExportFlowPerNodeId - Remote Node - Passed", "remote-node-id", "flow1", responseRemoteNode, nil},
		{"ExportFlowPerNodeId - Failed JSON Decoding", dataMgmtService.appConfig.NodeId, "flow1", &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`invalid json`)))}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error exporting flow flow1 from node")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			flow, err := exportFlowPerNodeId(dataMgmtService, tt.nodeId, tt.flowId)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
				assert.Equal(t, flow.Data, flowData)
				assert.NotEmpty(t, exportImportDataMock.FlowsData.Flows, "Expected flow to be exported")
			}
		})
	}
}

func TestDataMgmtService_exportRules(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportDataMock := dto.ExportImportData{
		RulesData: dto.ExportImportRules{
			Rules: []dto.Rule{
				{
					FromNodeId: "Node1",
					ToNodeId:   []string{"Node2"},
					Name:       []string{"Rule1"},
					Data: dto.RuleData{
						Id:  "rule-001",
						Sql: "SELECT * FROM data WHERE condition = true",
						Actions: []interface{}{
							"Action1",
							"Action2",
						},
						Options: map[string]interface{}{
							"option1": true,
							"option2": "value",
						},
					},
				},
				{
					FromNodeId: "Node2",
					ToNodeId:   []string{"Node3"},
					Name:       []string{"Rule2"},
					Data: dto.RuleData{
						Id:  "rule-002",
						Sql: "SELECT * FROM data WHERE condition = false",
						Actions: []interface{}{
							"Action3",
						},
						Options: nil,
					},
				},
			},
			FilePath: "/mock/path/rules.json",
			RulesIndexData: []map[string]interface{}{
				{
					"ruleId": "Rule1",
					"status": "active",
				},
				{
					"ruleId": "Rule2",
					"status": "inactive",
				},
			},
			Errors: nil,
		},
	}

	nodes := []dto.ExportNodeData{
		{
			NodeID: dataMgmtService.appConfig.NodeId,
			Ids:    []string{"rule1"},
		},
		{
			NodeID: "remote-node-id",
			Ids:    []string{"rule3"},
		},
		{
			NodeID: "node-id",
			Ids:    nil,
		},
	}
	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	expectedRuleData1 := dto.RuleData{Id: "rule3", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson1, _ := json.Marshal(expectedRuleData1)

	mockIndexFile, _ := os.CreateTemp("", "test-index-*.json")
	defer os.Remove(mockIndexFile.Name())

	ruleData := map[string]interface{}{
		"id":    "rule1",
		"label": "Test Rule",
	}
	ruleJsonData, _ := json.Marshal([]map[string]interface{}{ruleData})

	ruleData1 := map[string]interface{}{
		"id":    "rule3",
		"label": "Test Rule",
	}
	ruleJsonData1, _ := json.Marshal([]map[string]interface{}{ruleData1})

	tests := []struct {
		name          string
		node          dto.ExportNodeData
		expectedError error
	}{
		{"ExportRules - Local Node - Passed", nodes[0], nil},
		{"ExportRules - Remote Node - Passed", nodes[1], nil},
		{"ExportRules - Failed getRuleList", nodes[2], errors.New("The node should contain a list of rules ids")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient1 := &svcmocks.MockHTTPClient{}
			mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && req.URL.String() == "http://edgex-kuiper:59720/rules"
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(ruleJsonData)),
			}, nil)

			mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && req.URL.String() == "http://nats-proxy:48200/rules"
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(ruleJsonData1)),
			}, nil)

			mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && req.URL.String() == "http://edgex-kuiper:59720/rules/rule1"
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
			}, nil)

			mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && req.URL.String() == "http://nats-proxy:48200/rules/rule3"
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson1)),
			}, nil)

			client.Client = mockedHttpClient1

			err := dataMgmtService.exportRules([]dto.ExportNodeData{tt.node}, &exportImportDataMock.RulesData)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDataMgmtService_writeToIndexFile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	tests := []struct {
		name           string
		initialContent string
		key            string
		value          interface{}
		expectError    bool
		expectedError  string
		expectedData   map[string]interface{}
	}{
		{
			"WriteToIndexFile - Update Flows",
			`{"flows":[{"fromNodeId":"clm-aus-wpnk1l","id":"49844531321e1c8e","label":"Flow node1"}]}`,
			"flows",
			[]map[string]interface{}{
				{"fromNodeId": "clm-aus-wpnk1l", "id": "8a2e16035467de5a", "label": "Flow core 1"},
			},
			false,
			"",
			map[string]interface{}{
				"flows": []map[string]interface{}{
					{"fromNodeId": "clm-aus-wpnk1l", "id": "49844531321e1c8e", "label": "Flow node1"},
					{"fromNodeId": "clm-aus-wpnk1l", "id": "8a2e16035467de5a", "label": "Flow core 1"},
				},
			},
		},
		{
			"WriteToIndexFile - Add Rules",
			`{"rules":[{"fromNodeId":"clm-aus-wpnk1l","id":"LowGearOil_Delete_Rule"}]}`,
			"rules",
			[]map[string]interface{}{
				{"fromNodeId": "clm-aus-wpnk1l", "id": "TelcoLowFuelEvent_DeleteRule"},
			},
			false,
			"",
			map[string]interface{}{
				"rules": []map[string]interface{}{
					{"fromNodeId": "clm-aus-wpnk1l", "id": "LowGearOil_Delete_Rule"},
					{"fromNodeId": "clm-aus-wpnk1l", "id": "TelcoLowFuelEvent_DeleteRule"},
				},
			},
		},
		{
			"WriteToIndexFile - Invalid JSON Success",
			`invalid-json`,
			"flows",
			[]map[string]interface{}{
				{"fromNodeId": "clm-aus-wpnk1l", "id": "8a2e16035467de5a", "label": "Flow core 1"},
			},
			true,
			"Error writing to index file",
			nil,
		},
		{
			"WriteToIndexFile - Empty File Success",
			``,
			"flows",
			[]map[string]interface{}{
				{"fromNodeId": "clm-aus-wpnk1l", "id": "49844531321e1c8e", "label": "Flow node1"},
			},
			false,
			"",
			map[string]interface{}{
				"flows": []map[string]interface{}{
					{"fromNodeId": "clm-aus-wpnk1l", "id": "49844531321e1c8e", "label": "Flow node1"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := ioutil.TempFile("", "index*.json")
			assert.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			if tt.name != "WriteToIndexFile - Invalid JSON" {
				if tt.initialContent != "" {
					_, err = tmpFile.WriteString(tt.initialContent)
					assert.NoError(t, err)
				}
				tmpFile.Seek(0, 0) // Reset file pointer for reading
			} else {
				// Close the file to simulate file open error
				tmpFile.Close()
			}

			errHedge := dataMgmtService.writeToIndexFile(tmpFile, tt.key, tt.value)

			if tt.expectError {
				assert.NotNil(t, errHedge, "Expected an error but got nil")
				assert.Contains(t, errHedge.Message(), tt.expectedError, "Error message should contain the expected error")
			} else {
				assert.Nil(t, errHedge, "Expected no error but got one")

			}
		})
	}
}

func TestDataMgmtService_addFileToZip(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	tempFile, err := os.CreateTemp("", "testfile_*.txt")
	assert.Nil(t, err, "Expected no error when creating a temporary file")
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString("This is a test file content")
	assert.Nil(t, err, "Expected no error when writing to the temporary file")
	tempFilePath := tempFile.Name()

	tests := []struct {
		name         string
		filePath     string
		shouldFail   bool
		expectedErr  string
		zipFileExist bool
	}{
		{
			name:         "Success - Add valid file to zip",
			filePath:     tempFilePath,
			shouldFail:   false,
			zipFileExist: true,
		},
		{
			name:         "Fail - File does not exist",
			filePath:     "/nonexistent/path/testfile.txt",
			shouldFail:   true,
			expectedErr:  "Error adding file to zip file",
			zipFileExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			zipWriter := zip.NewWriter(&buffer)
			err := dataMgmtService.addFileToZip(zipWriter, tt.filePath)

			if tt.shouldFail {
				assert.NotNil(t, err, "Expected an error")
				assert.Contains(t, err.Error(), tt.expectedErr, "Expected error message to contain specific text")
			} else {
				assert.Nil(t, err, "Expected no error")
				// Close zipWriter and check the contents
				zipWriter.Close()

				zipReader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
				assert.Nil(t, err, "Expected no error when reading the zip content")
				assert.Equal(t, tt.zipFileExist, len(zipReader.File) > 0, "Expected the file to be added to the zip")
			}
		})
	}
}

func TestDataMgmtService_writeProfilesJsonFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validProfiles := []dto.ProfileObject{
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile1",
				},
			},
		},
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile2",
				},
			},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	fileName, hedgeError := writeProfilesJsonFile(dataMgmtService, baseTmpFolder, validProfiles)

	if hedgeError != nil {
		t.Errorf("writeProfilesJsonFile returned error, but expected success %v", hedgeError)
	}

	assert.Contains(t, fileName, filepath.Join(baseTmpFolder, PROFILES_FILE_NAME))
}

func TestDataMgmtService_writeProfilesJsonFile_ErrorCreatingFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validProfiles := []dto.ProfileObject{
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile1",
				},
			},
		},
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile2",
				},
			},
		},
	}

	_, hedgeError := writeProfilesJsonFile(dataMgmtService, baseTmpFolder, validProfiles)

	if hedgeError == nil {
		t.Errorf("writeProfilesJsonFile returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error exporting profiles: Error creating file")
	assert.Contains(t, hedgeError.Message(), expectedError.Message())
	assert.Equal(t, hedgeError.ErrorType(), expectedError.ErrorType())
}

func TestDataMgmtService_writeProfilesJsonFile_EmptyProfiles_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	var emptyProfiles []dto.ProfileObject

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	fileName, hedgeError := writeProfilesJsonFile(dataMgmtService, baseTmpFolder, emptyProfiles)

	if hedgeError != nil {
		t.Errorf("writeProfilesJsonFile returned error, but expected success %v", hedgeError)
	}

	assert.Contains(t, fileName, filepath.Join(baseTmpFolder, PROFILES_FILE_NAME))
}

func TestDataMgmtService_writeRulesJsonFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	rules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2", "node3"},
			Name:       []string{"Rule1"},
			Data: dto.RuleData{
				Id:      "rule1",
				Sql:     "SELECT * FROM table WHERE condition = true",
				Actions: []interface{}{"action1", "action2"},
				Options: map[string]interface{}{"opt1": "value1"},
			},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	fileName, hedgeError := writeRulesJsonFile(dataMgmtService, baseTmpFolder, rules)
	if hedgeError != nil {
		t.Errorf("writeRulesJsonFile returned error, but expected success %v", hedgeError)
	}

	assert.Contains(t, fileName, filepath.Join(baseTmpFolder, RULES_FILE_NAME))
}

func TestDataMgmtService_writeRulesJsonFile_ErrorCreatingFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	rules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2", "node3"},
			Name:       []string{"Rule1"},
			Data: dto.RuleData{
				Id:      "rule1",
				Sql:     "SELECT * FROM table WHERE condition = true",
				Actions: []interface{}{"action1", "action2"},
				Options: map[string]interface{}{"opt1": "value1"},
			},
		},
	}

	_, hedgeError := writeRulesJsonFile(dataMgmtService, baseTmpFolder, rules)
	if hedgeError == nil {
		t.Errorf("writeRulesJsonFile returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error exporting rules: Error creating file")
	assert.Contains(t, hedgeError.Message(), expectedError.Message())
	assert.Equal(t, hedgeError.ErrorType(), expectedError.ErrorType())
}

func TestDataMgmtService_writeFlowsJsonFile_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "flowNode1",
			ToNodeId:   []string{"flowNode2", "flowNode3"},
			Selected:   true,
			Data:       map[string]interface{}{"id": "flow1", "label": "Test Flow"},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	fileName, hedgeError := writeFlowsJsonFile(dataMgmtService, baseTmpFolder, flows)
	if hedgeError != nil {
		t.Errorf("writeFlowsJsonFile returned error, but expected success %v", hedgeError)
	}

	assert.Contains(t, fileName, filepath.Join(baseTmpFolder, FLOWS_FILE_NAME))
}

func TestDataMgmtService_writeFlowsJsonFile_ErrorCreatingFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "flowNode1",
			ToNodeId:   []string{"flowNode2", "flowNode3"},
			Selected:   true,
			Data:       map[string]interface{}{"id": "flow1", "label": "Test Flow"},
		},
	}

	_, hedgeError := writeFlowsJsonFile(dataMgmtService, baseTmpFolder, flows)
	if hedgeError == nil {
		t.Errorf("writeFlowsJsonFile returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error exporting flows: Error creating file")
	assert.Contains(t, hedgeError.Message(), expectedError.Message())
	assert.Equal(t, hedgeError.ErrorType(), expectedError.ErrorType())
}

func TestDataMgmtService_createListOfFilesPath(t *testing.T) {
	validExportData := dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			FilePath: "flows.json",
		},
		RulesData: dto.ExportImportRules{
			FilePath: "rules.json",
		},
		ProfilesData: dto.ExportImportProfiles{
			FilePath: "profiles.json",
		},
		IndexData: dto.ExportImportIndex{
			FilePath: "index.json",
		},
	}

	partialExportData := dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			FilePath: "",
		},
		RulesData: dto.ExportImportRules{
			FilePath: "rules.json",
		},
		ProfilesData: dto.ExportImportProfiles{
			FilePath: "",
		},
		IndexData: dto.ExportImportIndex{
			FilePath: "index.json",
		},
	}

	noFilesExportData := dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			FilePath: "",
		},
		RulesData: dto.ExportImportRules{
			FilePath: "",
		},
		ProfilesData: dto.ExportImportProfiles{
			FilePath: "",
		},
		IndexData: dto.ExportImportIndex{
			FilePath: "",
		},
	}

	tests := []struct {
		name          string
		exportData    dto.ExportImportData
		expectedPaths []string
		expectedErr   hedgeErrors.HedgeError
	}{
		{
			name:          "valid data - all file paths present",
			exportData:    validExportData,
			expectedPaths: []string{"flows.json", "rules.json", "profiles.json", "index.json"},
			expectedErr:   nil,
		},
		{
			name:          "valid data - some file paths missing",
			exportData:    partialExportData,
			expectedPaths: []string{"rules.json", "index.json"},
			expectedErr:   nil,
		},
		{
			name:          "no file paths present",
			exportData:    noFilesExportData,
			expectedPaths: nil,
			expectedErr:   hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "There is no data to export"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DataMgmtService{}
			paths, err := d.createListOfFilesPath(tt.exportData)
			if !reflect.DeepEqual(paths, tt.expectedPaths) {
				t.Errorf("expected paths %v, got %v", tt.expectedPaths, paths)
			}
			if !reflect.DeepEqual(err, tt.expectedErr) {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestDataMgmtService_getFlowsData_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validFlows := []dto.Flow{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Selected:   true,
			Data:       map[string]interface{}{"id": "flow1"},
		},
		{
			FromNodeId: "node2",
			ToNodeId:   []string{"node3"},
			Selected:   false,
			Data:       map[string]interface{}{"id": "flow2"},
		},
	}
	flowsDataJson, _ := json.Marshal(validFlows)

	fileName := filepath.Join(baseTmpFolder, "test.zip")
	_, _ = createTestZipFile(fileName, flowsDataJson)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	flows, hedgeError := dataMgmtService.getFlowsData(file)

	if hedgeError != nil {
		t.Errorf("getFlowsData returned error, but expected success %v", hedgeError)
	}
	if !reflect.DeepEqual(flows, validFlows) {
		t.Errorf("expected flows %+v, got %+v", validFlows, flows)
	}
}

func TestDataMgmtService_getFlowsData_ErrorOpeningFile_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	fileName := filepath.Join(baseTmpFolder, "test.zip")
	_, _ = createTestZipFile(fileName, []byte{})

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	_, hedgeError := dataMgmtService.getFlowsData(file)

	if hedgeError == nil {
		t.Errorf("getFlowsData returned error, but expected success: %v", hedgeError)
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting flows data from file %s: Error reading file %s", file.Name, file.Name))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_getFlowsData_InvalidJson_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	fileName := filepath.Join(baseTmpFolder, "test.zip")
	_, _ = createTestZipFile(fileName, []byte(`invalid-json`))

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	_, hedgeError := dataMgmtService.getFlowsData(file)

	if hedgeError == nil {
		t.Errorf("getFlowsData returned error, but expected success: %v", hedgeError)
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting flows data from file %s: Error reading file %s", file.Name, file.Name))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_getIndexData_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	indexData := dto.ExportImportIndex{
		Imports: dto.NodeData{
			ToNodeId: []string{"node1", "node2"},
			Flows:    []string{"flow1", "flow2"},
			Rules:    []string{"rule1", "rule2"},
			Profiles: []string{"profile1", "profile2"},
		},
		FilePath: "/path/to/index.json",
	}
	indexDataJson, _ := json.Marshal(indexData)

	fileName := filepath.Join(baseTmpFolder, "test_index.zip")
	_, _ = createTestZipFile(fileName, indexDataJson)

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	file := zipReader.File[0]

	actualData, hedgeError := dataMgmtService.getIndexData(file)

	if hedgeError != nil {
		t.Errorf("getIndexData returned error, but expected success: %v", hedgeError)
	}
	if !reflect.DeepEqual(actualData, indexData) {
		t.Errorf("expected index %+v, got %+v", indexData, actualData)
	}
}

func TestDataMgmtService_getIndexData_InvalidJson_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	fileName := filepath.Join(baseTmpFolder, "test_index.zip")
	_, _ = createTestZipFile(fileName, []byte(`invalid-json`))

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	file := zipReader.File[0]

	_, hedgeError := dataMgmtService.getIndexData(file)

	if hedgeError == nil {
		t.Errorf("getIndexData returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting index data from file  %s: Error reading file %s", file.Name, file.Name))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeError, expectedError)
	}

}

func TestDataMgmtService_getRulesData_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validRules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Name:       []string{"rule1"},
			Data: dto.RuleData{
				Id:      "rule-id-1",
				Sql:     "SELECT * FROM data WHERE id=1",
				Actions: []interface{}{"action1"},
				Options: nil,
			},
		},
		{
			FromNodeId: "node2",
			ToNodeId:   []string{"node3"},
			Name:       []string{"rule2"},
			Data: dto.RuleData{
				Id:      "rule-id-2",
				Sql:     "SELECT * FROM data WHERE id=2",
				Actions: []interface{}{"action2"},
				Options: nil,
			},
		},
	}
	rulesDataJson, _ := json.Marshal(validRules)

	fileName := filepath.Join(baseTmpFolder, "test_rules.zip")
	_, _ = createTestZipFile(fileName, rulesDataJson)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	rules, hedgeError := dataMgmtService.getRulesData(file)
	if hedgeError != nil {
		t.Errorf("getRulesData returned success, but expected error")
	}
	if !reflect.DeepEqual(rules, validRules) {
		t.Errorf("expected profiles %+v, got %+v", validRules, rules)
	}
}

func TestDataMgmtService_getRulesData_InvalidJson_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	fileName := filepath.Join(baseTmpFolder, "test_rules.zip")
	_, _ = createTestZipFile(fileName, []byte(`invalid-json`))
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	_, hedgeError := dataMgmtService.getRulesData(file)
	if hedgeError == nil {
		t.Errorf("getRulesData returned error, but expected success: %v", hedgeError)
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting rules data from file %s: Error reading file %s", file.Name, file.Name))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_getProfilesData_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validProfiles := []dto.ProfileObject{
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile1",
				},
			},
		},
		{
			Profile: dtos.DeviceProfile{
				DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
					Name: "profile2",
				},
			},
		},
	}
	profilesDataJson, _ := json.Marshal(validProfiles)

	fileName := filepath.Join(baseTmpFolder, "test_profiles.zip")
	_, _ = createTestZipFile(fileName, profilesDataJson)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	profiles, hedgeError := dataMgmtService.getProfilesData(file)
	if hedgeError != nil {
		t.Errorf("getProfilesData returned success, but expected error")
	}
	if !reflect.DeepEqual(profiles, validProfiles) {
		t.Errorf("expected profiles %+v, got %+v", validProfiles, profiles)
	}
}

func TestDataMgmtService_getProfilesData_InvalidJson_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	fileName := filepath.Join(baseTmpFolder, "test_profiles.zip")
	_, _ = createTestZipFile(fileName, []byte(`invalid-json`))
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	zipReader, _ := zip.OpenReader(fileName)
	defer zipReader.Close()

	file := zipReader.File[0]

	_, hedgeError := dataMgmtService.getProfilesData(file)
	if hedgeError == nil {
		t.Errorf("getProfilesData returned error, but expected success: %v", hedgeError)
	}
	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error getting profiles data from file %s: Error reading file %s", file.Name, file.Name))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match.\n Recieved:\n%v\n Expected:\n%v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_updateImportFlowData(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validFlows := []dto.Flow{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Selected:   false,
			Data:       map[string]interface{}{"id": "flow1"},
		},
		{
			FromNodeId: "node2",
			ToNodeId:   []string{"node3"},
			Selected:   true,
			Data:       map[string]interface{}{"id": "flow2"},
		},
		{
			FromNodeId: "node3",
			ToNodeId:   []string{"node4"},
			Selected:   false,
			Data:       map[string]interface{}{"id": "flow3"},
		},
	}

	importData := dto.ExportImportData{
		IndexData: dto.ExportImportIndex{
			Imports: dto.NodeData{
				ToNodeId: []string{"node5"},
				Flows:    []string{"flow1", "flow3"},
			},
		},
		FlowsData: dto.ExportImportFlow{
			Flows: validFlows,
		},
	}

	tests := []struct {
		name          string
		importData    *dto.ExportImportData
		expectedFlows []dto.Flow
	}{
		{
			name:       "successful flow data update",
			importData: &importData,
			expectedFlows: []dto.Flow{
				{
					FromNodeId: "node1",
					ToNodeId:   []string{"node5"},
					Selected:   true,
					Data:       map[string]interface{}{"id": "flow1"},
				},
				{
					FromNodeId: "node3",
					ToNodeId:   []string{"node5"},
					Selected:   true,
					Data:       map[string]interface{}{"id": "flow3"},
				},
			},
		},
		{
			name: "no flows to update",
			importData: &dto.ExportImportData{
				IndexData: dto.ExportImportIndex{
					Imports: dto.NodeData{
						ToNodeId: []string{"node6"},
						Flows:    []string{},
					},
				},
				FlowsData: dto.ExportImportFlow{
					Flows: validFlows,
				},
			},
			expectedFlows: []dto.Flow{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataMgmtService.updateImportFlowData(tt.importData)
			assert.Equal(t, tt.expectedFlows, tt.importData.FlowsData.Flows, "Expected flows do not match the actual flows.")
		})
	}
}

func TestDataMgmtService_updateImportRulesData(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	validRules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Name:       []string{"Rule1"},
			Data:       dto.RuleData{Id: "rule1", Sql: "SELECT * FROM table1", Actions: nil, Options: nil},
		},
		{
			FromNodeId: "node2",
			ToNodeId:   []string{"node3"},
			Name:       []string{"Rule2"},
			Data:       dto.RuleData{Id: "rule2", Sql: "SELECT * FROM table2", Actions: nil, Options: nil},
		},
		{
			FromNodeId: "node3",
			ToNodeId:   []string{"node4"},
			Name:       []string{"Rule3"},
			Data:       dto.RuleData{Id: "rule3", Sql: "SELECT * FROM table3", Actions: nil, Options: nil},
		},
	}

	importData := &dto.ExportImportData{
		IndexData: dto.ExportImportIndex{
			Imports: dto.NodeData{
				ToNodeId: []string{"node5"},
				Rules:    []string{"rule1", "rule3"},
			},
		},
		RulesData: dto.ExportImportRules{
			Rules: validRules,
		},
	}

	tests := []struct {
		name          string
		importData    *dto.ExportImportData
		expectedRules []dto.Rule
	}{
		{
			name:       "successful rule data update",
			importData: importData,
			expectedRules: []dto.Rule{
				{
					FromNodeId: "node1",
					ToNodeId:   []string{"node5"},
					Name:       []string{"Rule1"},
					Data:       dto.RuleData{Id: "rule1", Sql: "SELECT * FROM table1", Actions: nil, Options: nil},
				},
				{
					FromNodeId: "node3",
					ToNodeId:   []string{"node5"},
					Name:       []string{"Rule3"},
					Data:       dto.RuleData{Id: "rule3", Sql: "SELECT * FROM table3", Actions: nil, Options: nil},
				},
			},
		},
		{
			name: "no rules to update",
			importData: &dto.ExportImportData{
				IndexData: dto.ExportImportIndex{
					Imports: dto.NodeData{
						ToNodeId: []string{"node6"},
						Rules:    []string{},
					},
				},
				RulesData: dto.ExportImportRules{
					Rules: validRules,
				},
			},
			expectedRules: []dto.Rule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataMgmtService.updateImportRulesData(tt.importData)

			assert.Equal(t, tt.expectedRules, tt.importData.RulesData.Rules, "Expected rules do not match the actual rules.")
		})
	}
}

func TestDataMgmtService_importFlows(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "Node0",
			ToNodeId:   []string{dataMgmtService.appConfig.NodeId},
			Selected:   true,
			Data: map[string]interface{}{
				"id":    "flow1",
				"label": "Test Flow",
			},
		},
		{
			FromNodeId: "Node1",
			ToNodeId:   []string{"remote-node-id"},
			Selected:   true,
			Data: map[string]interface{}{
				"id":    "flow2",
				"label": "Test Flow",
			},
		},
		{
			FromNodeId: "Node2",
			ToNodeId:   []string{"remote-node-id1"},
			Selected:   false,
			Data: map[string]interface{}{
				"id":    "flow3",
				"label": "Test Flow",
			},
		},
	}

	response1 := &http.Response{
		StatusCode: http.StatusOK,
	}
	response2 := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name               string
		node               string
		response           *http.Response
		expectedError      hedgeErrors.HedgeError
		expectedFlowsError []dto.NodeError
	}{
		{
			name:     "ImportFlows - Local Node - Passed",
			node:     flows[0].ToNodeId[0],
			response: response1,
		},
		{
			name:     "ImportFlows - Remote Node - Passed",
			node:     flows[1].ToNodeId[0],
			response: response2,
		},
		{
			name:     "ImportFlows - Multiple Nodes - Failed",
			node:     "",
			response: nil,
			expectedError: hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeBadRequest, "Error importing flow"),
			expectedFlowsError: []dto.NodeError{
				{
					NodeID: "",
					Errors: []hedgeErrors.HedgeError{
						hedgeErrors.NewCommonHedgeError(
							hedgeErrors.ErrorTypeBadRequest, "Error importing flow with ID: flow1"),
					},
				},
				{
					NodeID: "remote-node-id",
					Errors: []hedgeErrors.HedgeError{
						hedgeErrors.NewCommonHedgeError(
							hedgeErrors.ErrorTypeBadRequest, "Error importing flow with ID: flow2"),
					},
				},
				{
					NodeID: "remote-node-id1",
					Errors: []hedgeErrors.HedgeError{
						hedgeErrors.NewCommonHedgeError(
							hedgeErrors.ErrorTypeBadRequest, "Error importing flow with ID: flow3"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			var flowsError []dto.NodeError
			dataMgmtService.importFlows(flows, &flowsError)

			if tt.expectedError != nil {
				assert.NotNil(t, tt.expectedError)
				assert.Equal(t, tt.expectedFlowsError, flowsError, "Unexpected flowsError content")
			} else {
				assert.Empty(t, flowsError, "Expected no errors in flowsError")
			}
		})
	}
}

func TestDataMgmtService_importFlowPerNodeId(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "Node0",
			ToNodeId:   []string{dataMgmtService.appConfig.NodeId},
			Selected:   true,
			Data: map[string]interface{}{
				"id":    "flow1",
				"label": "Test Flow",
			},
		},
		{
			FromNodeId: "Node2",
			ToNodeId:   []string{"remote-node-id1"},
			Selected:   false,
			Data: map[string]interface{}{
				"id":    func() {},
				"label": "Test Flow",
			},
		},
	}

	response := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		nodeId        string
		flow          dto.Flow
		response      *http.Response
		expectedError error
	}{
		{"ImportFlowPerNodeId - Local Node - Passed", dataMgmtService.appConfig.NodeId, flows[0], response, nil},
		{"ImportFlowPerNodeId - Remote Node - Passed", "remote-node-id", flows[0], response, nil},
		{"ImportFlowPerNodeId - Failed JSON Decoding", dataMgmtService.appConfig.NodeId, flows[1], &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`invalid json`)))}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error importing flow")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			err := importFlowPerNodeId(dataMgmtService, tt.flow, tt.nodeId)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDataMgmtService_importRules(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	rules := []dto.Rule{
		{
			FromNodeId: "Node1",
			ToNodeId:   []string{"remote-node-id1"},
			Name:       []string{"Rule1"},
			Data: dto.RuleData{
				Id:  "rule-001",
				Sql: "SELECT * FROM data WHERE condition = true",
				Actions: []interface{}{
					"Action1",
					"Action2",
				},
				Options: map[string]interface{}{
					"option1": true,
					"option2": "value",
				},
			},
		},
		{
			FromNodeId: "Node2",
			ToNodeId:   []string{"remote-node-id1"},
			Name:       []string{"Rule2"},
			Data: dto.RuleData{
				Id:  "rule-002",
				Sql: "SELECT * FROM data WHERE condition = false",
				Actions: []interface{}{
					"Action3",
				},
				Options: nil,
			},
		},
	}

	response1 := &http.Response{
		StatusCode: http.StatusOK,
	}
	response2 := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		node          string
		response      *http.Response
		expectedError error
	}{
		{"ImportRules - Local Node - Passed", rules[0].ToNodeId[0], response1, nil},
		{"ImportRules - Remote Node - Passed", rules[1].ToNodeId[0], response2, nil},
		{"ImportRules - Failed Empty node ids list", rules[1].ToNodeId[0], nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error importing rule")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			var rulesError []dto.NodeError
			dataMgmtService.importRules(rules, &rulesError)

			if tt.expectedError != nil {
				assert.NotEmpty(t, rulesError)
				found := false
				for _, nodeError := range rulesError {
					if nodeError.NodeID == tt.node {
						found = true
						assert.Contains(t, nodeError.Errors[0].Error(), tt.expectedError.Error())
					}
				}
				assert.True(t, found, "Expected error for node %s not found", tt.node)
			} else {
				assert.Empty(t, rulesError)
			}
		})
	}
}

func TestDataMgmtService_importRulePerNodeId(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	rules := []dto.Rule{
		{
			FromNodeId: "Node1",
			ToNodeId:   []string{"remote-node-id1"},
			Name:       []string{"Rule1"},
			Data: dto.RuleData{
				Id:  "rule-001",
				Sql: "SELECT * FROM data WHERE condition = true",
				Actions: []interface{}{
					"Action1",
					"Action2",
				},
				Options: map[string]interface{}{
					"option1": true,
					"option2": "value",
				},
			},
		},
		{
			FromNodeId: "Node2",
			ToNodeId:   []string{"remote-node-id1"},
			Name:       []string{"Rule2"},
			Data: dto.RuleData{
				Id:  "rule-002",
				Sql: "SELECT * FROM data WHERE condition = false",
				Actions: []interface{}{
					"Action3",
				},
				Options: func() {},
			},
		},
	}

	response := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		nodeId        string
		rule          dto.Rule
		response      *http.Response
		expectedError error
	}{
		{"ImportRulePerNodeId - Local Node - Passed", dataMgmtService.appConfig.NodeId, rules[0], response, nil},
		{"ImportRulePerNodeId - Remote Node - Passed", "remote-node-id", rules[0], response, nil},
		{"ImportRulePerNodeId - Failed JSON Decoding", dataMgmtService.appConfig.NodeId, rules[1], &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`invalid json`)))}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error importing rule")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			err := importRulePerNodeId(dataMgmtService, tt.rule.Data, tt.nodeId)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDataMgmtService_importProfiles(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportImportDataMock := dto.ExportImportData{
		IndexData: dto.ExportImportIndex{
			Imports: dto.NodeData{
				Profiles: []string{"profile1", "profile2"},
			},
			FilePath: "/mock/path/index.json",
		},
		ProfilesData: dto.ExportImportProfiles{
			Profiles: []dto.ProfileObject{
				{
					Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "test-id1", Name: "profile1"}},
				},
				{
					Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "test-id2", Name: "profile2"}},
				},
			},
			FilePath: "/mock/path/profiles.json",
			ProfilesIndexData: []string{
				"profile1",
				"profile2",
			},
			Errors: nil,
		},
	}

	response1 := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		response      *http.Response
		expectedError error
	}{
		{"ImportProfiles - Passed", response1, nil},
		{"ImportProfiles - Failed", nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "failed to add profiles")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			var profilesError []dto.ProfileError
			dataMgmtService.importProfiles(
				exportImportDataMock.ProfilesData.Profiles,
				exportImportDataMock.IndexData.Imports.Profiles,
				&profilesError,
			)

			if tt.expectedError != nil {
				assert.NotNil(t, profilesError)
				found := false
				for _, profileError := range profilesError {
					for _, err := range profileError.Errors {
						if strings.Contains(err.Error(), tt.expectedError.Error()) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				assert.True(t, found, "Expected error not found in profilesError")
			} else {
				assert.Empty(t, profilesError, "Expected no errors in profilesError")
			}
		})
	}
}

func TestDataMgmtService_importProfile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	profileSuccess := dto.ProfileObject{Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}}}
	profileFail := dto.ProfileObject{
		ApiVersion: "1.0",
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: "profile1",
			},
		},
	}
	invalidData := map[string]interface{}{
		"invalidField": func() {},
	}
	profileFail.Profile.DeviceResources = append(profileFail.Profile.DeviceResources, dtos.DeviceResource{
		Attributes: invalidData,
	})
	response := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		profile       dto.ProfileObject
		response      *http.Response
		expectedError error
	}{
		{"ImportProfilePerNodeId - Passed", profileSuccess, response, nil},
		{"ImportProfilePerNodeId - Failed JSON Decoding", profileFail, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`invalid json`)))}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error importing profiles")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient := &svcmocks.MockHTTPClient{}
			if tt.response != nil {
				mockedHttpClient.On("Do", mock.Anything).Return(tt.response, nil)
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("request failed"))
			}
			client.Client = mockedHttpClient

			err := importProfile(dataMgmtService, tt.profile)

			if tt.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDataMgmtService_createJsonFiles_Flows_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Data:       map[string]interface{}{"id": "flow1"},
		},
	}

	exportData := &dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			Flows:  flows,
			Errors: []dto.NodeError{},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	dataMgmtService.createJsonFiles(baseTmpFolder, exportData)

	if len(exportData.FlowsData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for FlowsData, but expected success: %v", exportData.FlowsData.Errors)
	}
	assert.Contains(t, exportData.FlowsData.FilePath, filepath.Join(baseTmpFolder, FLOWS_FILE_NAME))
}

func TestDataMgmtService_createJsonFiles_Profiles_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	profiles := []dto.ProfileObject{
		{
			Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}},
		},
	}

	exportData := &dto.ExportImportData{
		ProfilesData: dto.ExportImportProfiles{
			Profiles: profiles,
			Errors:   []dto.ProfileError{},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	dataMgmtService.createJsonFiles(baseTmpFolder, exportData)

	if len(exportData.ProfilesData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for ProfilesData, but expected success: %v", exportData.ProfilesData.Errors)
	}
	assert.Contains(t, exportData.ProfilesData.FilePath, filepath.Join(baseTmpFolder, PROFILES_FILE_NAME))
}

func TestDataMgmtService_createJsonFiles_All_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flows := []dto.Flow{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Data:       map[string]interface{}{"id": "flow1"},
		},
	}

	rules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Name:       []string{"rule1"},
			Data: dto.RuleData{
				Id:      "rule1",
				Sql:     "SELECT * FROM table",
				Actions: []interface{}{"action1"},
			},
		},
	}

	profiles := []dto.ProfileObject{
		{
			Profile: dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Name: "profile1"}},
		},
	}

	exportData := &dto.ExportImportData{
		FlowsData: dto.ExportImportFlow{
			Flows:  flows,
			Errors: []dto.NodeError{},
		},
		RulesData: dto.ExportImportRules{
			Rules:  rules,
			Errors: []dto.NodeError{},
		},
		ProfilesData: dto.ExportImportProfiles{
			Profiles: profiles,
			Errors:   []dto.ProfileError{},
		},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	dataMgmtService.createJsonFiles(baseTmpFolder, exportData)

	if len(exportData.ProfilesData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for ProfilesData, but expected success: %v", exportData.ProfilesData.Errors)
	}
	assert.Contains(t, exportData.ProfilesData.FilePath, filepath.Join(baseTmpFolder, PROFILES_FILE_NAME))

	if len(exportData.RulesData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for RulesData, but expected success: %v", exportData.RulesData.Errors)
	}
	assert.Contains(t, exportData.RulesData.FilePath, filepath.Join(baseTmpFolder, RULES_FILE_NAME))

	if len(exportData.FlowsData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for FlowsData, but expected success: %v", exportData.FlowsData.Errors)
	}
	assert.Contains(t, exportData.FlowsData.FilePath, filepath.Join(baseTmpFolder, FLOWS_FILE_NAME))
}

func TestDataMgmtService_createJsonFiles_EmptyData_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	exportData := &dto.ExportImportData{
		FlowsData:    dto.ExportImportFlow{Errors: []dto.NodeError{}},
		RulesData:    dto.ExportImportRules{Errors: []dto.NodeError{}},
		ProfilesData: dto.ExportImportProfiles{Errors: []dto.ProfileError{}},
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	dataMgmtService.createJsonFiles(baseTmpFolder, exportData)

	if len(exportData.ProfilesData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for ProfilesData, but expected success: %v", exportData.ProfilesData.Errors)
	}
	assert.Contains(t, exportData.ProfilesData.FilePath, "")

	if len(exportData.RulesData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for RulesData, but expected success: %v", exportData.RulesData.Errors)
	}
	assert.Contains(t, exportData.RulesData.FilePath, "")

	if len(exportData.FlowsData.Errors) > 0 {
		t.Errorf("createJsonFiles returned errors for FlowsData, but expected success: %v", exportData.FlowsData.Errors)
	}
	assert.Contains(t, exportData.FlowsData.FilePath, "")
}
func TestDataMgmtService_parseZipFile(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	e := echo.New()

	tests := []struct {
		name         string
		indexData    interface{}
		flowsData    []dto.Flow
		rulesData    []dto.Rule
		profilesData []dto.ProfileObject
		expectError  bool
	}{
		{
			name: "Valid data",
			indexData: dto.ExportImportIndex{
				Imports: dto.NodeData{
					ToNodeId: []string{"node1", "node2"},
					Flows:    []string{"flow1", "flow2"},
					Rules:    []string{"rule1"},
					Profiles: []string{"profile1"},
				},
				FilePath: "test/index.json",
			},
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: true, Data: map[string]interface{}{"flowProperty": "someValue"}},
			},
			rulesData: []dto.Rule{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Name: []string{"rule1"}, Data: dto.RuleData{Id: "rule1", Sql: "SELECT * FROM sensor_data WHERE value > 50", Actions: nil, Options: nil}},
			},
			profilesData: []dto.ProfileObject{
				{Profile: dtos.DeviceProfile{
					DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
						Name: "profile1",
					},
				},
				}},
			expectError: false,
		},
		{
			name:      "Invalid index data",
			indexData: "",
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: true, Data: nil},
			},
			rulesData:    []dto.Rule{},
			profilesData: []dto.ProfileObject{},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			zipBuffer, err := createTestZipFileForParseZipFile(tt.indexData, tt.flowsData, tt.rulesData, tt.profilesData)
			assert.NoError(t, err)

			part, err := writer.CreateFormFile("file", "test.zip")
			assert.NoError(t, err)

			_, err = io.Copy(part, zipBuffer)
			assert.NoError(t, err)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			MaxImportExportFileSizeBytes := int64(10 * 1024 * 1024) // 10MB limit
			c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, MaxImportExportFileSizeBytes)

			file, err := c.FormFile("file")
			assert.NoError(t, err)

			parsedData, hedgeErr := dataMgmtService.ParseZipFile(file)

			if tt.expectError {
				assert.Error(t, hedgeErr)
				assert.Equal(t, parsedData, dto.ExportImportData{})
			} else {
				assert.NoError(t, hedgeErr)
				assert.NotNil(t, parsedData)

				assert.Equal(t, "node1", parsedData.IndexData.Imports.ToNodeId[0])
				assert.Equal(t, "flow1", parsedData.IndexData.Imports.Flows[0])
				assert.Equal(t, "rule1", parsedData.IndexData.Imports.Rules[0])
				assert.Equal(t, "profile1", parsedData.IndexData.Imports.Profiles[0])

				assert.Equal(t, "node1", parsedData.FlowsData.Flows[0].FromNodeId)
				assert.True(t, parsedData.FlowsData.Flows[0].Selected)

				assert.Equal(t, "rule1", parsedData.RulesData.Rules[0].Data.Id)
				assert.Equal(t, "SELECT * FROM sensor_data WHERE value > 50", parsedData.RulesData.Rules[0].Data.Sql)

				assert.Equal(t, "profile1", parsedData.ProfilesData.Profiles[0].Profile.Name)
			}
		})
	}
}

func TestDataMgmtService_ExportData(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	flowsDataMap := map[string]interface{}{
		"id": "flow1", "label": "Test Flow",
	}

	flowsDataObject := []dto.ExportNodeData{
		{NodeID: "node1", Host: "host1", Ids: []string{"flow1"}},
	}

	flowsDataObjectEmptyIds := []dto.ExportNodeData{
		{NodeID: "node1", Host: "host1", Ids: nil},
	}

	rulesDataMap := []map[string]interface{}{
		{"id": "rule1", "label": "Test Rule"},
	}

	rulesDataObject := []dto.ExportNodeData{
		{NodeID: "node1", Host: "host1", Ids: []string{"rule1"}},
	}

	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	flowJsonData, _ := json.Marshal(flowsDataMap)

	rulesJsonData, _ := json.Marshal(rulesDataMap)

	profilesData := []string{"profile1"}
	profileName := "profile1"
	profilesJsonObject := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	profilesJsonData, _ := json.Marshal(profilesJsonObject)

	tests := []struct {
		name         string
		requestData  dto.NodeReqData
		expectedData dto.ExportImportData
		expectError  bool
	}{
		{
			name: "All fields populated successfully",
			requestData: dto.NodeReqData{
				Flows:    flowsDataObject,
				Rules:    rulesDataObject,
				Profiles: profilesData,
			},
			expectedData: dto.ExportImportData{
				FlowsData: dto.ExportImportFlow{
					Flows: []dto.Flow{
						{FromNodeId: "node1", ToNodeId: nil, Selected: false, Data: map[string]interface{}{"id": "flow1", "label": "Test Flow"}},
					},
					FlowsIndexData: []map[string]interface{}{
						{
							"id":         "flow1",
							"fromNodeId": "node1",
							"label":      "Test Flow",
						},
					},
				},
				RulesData: dto.ExportImportRules{
					Rules: []dto.Rule{
						{FromNodeId: "node1", ToNodeId: nil, Name: nil, Data: dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}},
					},
					RulesIndexData: []map[string]interface{}{
						{
							"id":         "rule1",
							"fromNodeId": "node1",
						},
					},
				},
				ProfilesData: dto.ExportImportProfiles{
					Profiles: []dto.ProfileObject{
						{
							Profile: dtos.DeviceProfile{
								DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
									Name: profileName,
								},
							},
						},
					},
					ProfilesIndexData: profilesData,
				},
			},
			expectError: false,
		},
		{
			name: "Error when exporting flows",
			requestData: dto.NodeReqData{
				Flows:    flowsDataObjectEmptyIds,
				Rules:    rulesDataObject,
				Profiles: profilesData,
			},
			expectedData: dto.ExportImportData{
				FlowsData: dto.ExportImportFlow{
					Errors: []dto.NodeError{
						{
							Errors: []hedgeErrors.HedgeError{
								hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "The node should contain a list of workflow ids"),
							},
						},
					},
				},
				RulesData: dto.ExportImportRules{
					Rules: []dto.Rule{
						{FromNodeId: "node1", ToNodeId: nil, Name: nil, Data: dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}},
					},
					RulesIndexData: []map[string]interface{}{
						{
							"id":         "rule1",
							"fromNodeId": "node1",
						},
					},
				},
				ProfilesData: dto.ExportImportProfiles{
					Profiles: []dto.ProfileObject{
						{
							Profile: dtos.DeviceProfile{
								DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
									Name: profileName,
								},
							},
						},
					},
					ProfilesIndexData: profilesData,
				},
			},
			expectError: true,
		},
		{
			name: "No data to export",
			requestData: dto.NodeReqData{
				Flows:    []dto.ExportNodeData{},
				Rules:    []dto.ExportNodeData{},
				Profiles: []string{},
			},
			expectedData: dto.ExportImportData{
				FlowsData:    dto.ExportImportFlow{},
				RulesData:    dto.ExportImportRules{},
				ProfilesData: dto.ExportImportProfiles{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		flowsResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
		}

		rulesResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
		}

		profilesResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(profilesJsonData)),
		}
		mockedHttpClient := &svcmocks.MockHTTPClient{}

		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return strings.Contains(req.URL.String(), "http://"+NATSPROXY+"/flow/flow1")
		})).Return(flowsResponse, nil)

		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet && req.URL.String() == "http://"+KUIPER+"/rules"
		})).Return(rulesResponse, nil)

		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules"
		})).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
		}, nil)
		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules/rule1"
		})).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
		}, nil)
		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet && req.URL.String() == "http://"+KUIPER+"/rules/rule1"
		})).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
		}, nil)
		mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Method == http.MethodGet &&
				req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
		})).Return(profilesResponse, nil)
		client.Client = mockedHttpClient
		t.Run(tt.name, func(t *testing.T) {

			result := dataMgmtService.ExportData(tt.requestData)

			if tt.expectError {
				assert.NotEmpty(t, result.FlowsData.Errors)
			} else {
				assert.Empty(t, result.FlowsData.Errors)
				assert.Empty(t, result.RulesData.Errors)
				assert.Empty(t, result.ProfilesData.Errors)
			}

			assert.Equal(t, tt.expectedData, result)
		})
	}
}

func TestDataMgmtService_PrepareImportData(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	testFlows := []dto.Flow{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Selected:   false,
			Data:       map[string]interface{}{"id": "flow1", "label": "Test Flow"},
		},
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node3"},
			Selected:   false,
			Data:       map[string]interface{}{"id": "flow2", "label": "Test Flow"},
		},
	}

	testRules := []dto.Rule{
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node2"},
			Name:       []string{"rule1"},
			Data: dto.RuleData{
				Id:      "rule1",
				Sql:     "SELECT * FROM table",
				Actions: []interface{}{"action1"},
				Options: nil,
			},
		},
		{
			FromNodeId: "node1",
			ToNodeId:   []string{"node3"},
			Name:       []string{"rule2"},
			Data: dto.RuleData{
				Id:      "rule2",
				Sql:     "SELECT * FROM table",
				Actions: []interface{}{"action2"},
				Options: nil,
			},
		},
	}

	testIndexData := dto.ExportImportIndex{
		Imports: dto.NodeData{
			ToNodeId: []string{"new-node1"},
			Flows:    []string{"flow1"},
			Rules:    []string{"rule1"},
		},
	}

	importData := &dto.ExportImportData{
		IndexData: testIndexData,
		FlowsData: dto.ExportImportFlow{
			Flows: testFlows,
		},
		RulesData: dto.ExportImportRules{
			Rules: testRules,
		},
	}

	tests := []struct {
		name          string
		importData    *dto.ExportImportData
		expectedFlows []dto.Flow
		expectedRules []dto.Rule
		expectError   bool
		expectedError string
	}{
		{
			name:       "Prepare Import Data - Success",
			importData: importData,
			expectedFlows: []dto.Flow{
				{
					FromNodeId: "node1",
					ToNodeId:   []string{"new-node1"},
					Selected:   true,
					Data:       map[string]interface{}{"id": "flow1", "label": "Test Flow"},
				},
			},
			expectedRules: []dto.Rule{
				{
					FromNodeId: "node1",
					ToNodeId:   []string{"new-node1"},
					Name:       []string{"rule1"},
					Data: dto.RuleData{
						Id:      "rule1",
						Sql:     "SELECT * FROM table",
						Actions: []interface{}{"action1"},
						Options: nil,
					},
				},
			},
			expectError: false,
		},
		{
			name:          "Prepare Import Data - Nil ImportData",
			importData:    nil,
			expectError:   true,
			expectedError: "Failed to Import all requested data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dataMgmtService.PrepareImportData(tt.importData)

			if tt.expectError {
				assert.NotNil(t, err, "Expected an error but got nil")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message should match the expected error")
			} else {
				assert.Nil(t, err, "Expected no error but got one")

				if !reflect.DeepEqual(tt.importData.FlowsData.Flows, tt.expectedFlows) {
					t.Errorf("FlowsData mismatch. Got: %v, expected: %v", tt.importData.FlowsData.Flows, tt.expectedFlows)
				}

				if !reflect.DeepEqual(tt.importData.RulesData.Rules, tt.expectedRules) {
					t.Errorf("RulesData mismatch. Got: %v, expected: %v", tt.importData.RulesData.Rules, tt.expectedRules)
				}
			}
		})
	}
}

func TestDataMgmtService_ExportDefs_ValidDataSameNode_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	dataMgmtService.appConfig.NodeId = "node-1"

	mockedHttpClient := &svcmocks.MockHTTPClient{}

	flows := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"flow-1"},
		},
	}

	rules := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"rule-1"},
		},
	}

	profiles := []string{"profile-1"}

	requestData := dto.NodeReqData{
		Flows:    flows,
		Rules:    rules,
		Profiles: profiles,
	}

	flowsDataMap := map[string]interface{}{
		"id": "flow-1", "label": "Test Flow",
	}

	rulesDataMap := []map[string]interface{}{
		{"id": "rule-1", "label": "Test Rule"},
	}

	flowJsonData, _ := json.Marshal(flowsDataMap)

	rulesJsonData, _ := json.Marshal(rulesDataMap)

	profileName := "profile-1"
	profilesJsonObject := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	profilesJsonData, _ := json.Marshal(profilesJsonObject)

	flowsResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	rulesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
	}

	profilesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(profilesJsonData)),
	}

	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "http://"+NODERED+"/flow/flow-1")
	})).Return(flowsResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+KUIPER+"/rules"
	})).Return(rulesResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+KUIPER+"/rules/rule-1"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
	}, nil)
	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
	})).Return(profilesResponse, nil)
	client.Client = mockedHttpClient

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)
	if hedgeError != nil {
		t.Errorf("ExportDefs returned error, but expected success %v", hedgeError)
	}

	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", filePath)
	}
}

func TestDataMgmtService_ExportDefs_ValidDataDifferentNode_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	dataMgmtService.appConfig.NodeId = "node-2"

	mockedHttpClient := &svcmocks.MockHTTPClient{}

	flows := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"flow-1"},
		},
	}

	rules := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"rule-1"},
		},
	}

	profiles := []string{"profile-1"}

	requestData := dto.NodeReqData{
		Flows:    flows,
		Rules:    rules,
		Profiles: profiles,
	}

	flowsDataMap := map[string]interface{}{
		"id": "flow-1", "label": "Test Flow",
	}

	rulesDataMap := []map[string]interface{}{
		{"id": "rule-1", "label": "Test Rule"},
	}

	flowJsonData, _ := json.Marshal(flowsDataMap)

	rulesJsonData, _ := json.Marshal(rulesDataMap)

	profileName := "profile-1"
	profilesJsonObject := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	profilesJsonData, _ := json.Marshal(profilesJsonObject)

	flowsResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	rulesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
	}

	profilesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(profilesJsonData)),
	}

	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "http://"+NATSPROXY+"/flow/flow-1")
	})).Return(flowsResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules"
	})).Return(rulesResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules/rule-1"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
	}, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
	})).Return(profilesResponse, nil)
	client.Client = mockedHttpClient

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)

	if hedgeError != nil {
		t.Errorf("ExportDefs returned error, but expected success %v", hedgeError)
	}

	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", filePath)
	}
}

func TestDataMgmtService_ExportDefs_NoFlows_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	mockedHttpClient := &svcmocks.MockHTTPClient{}

	rules := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"rule-1"},
		},
	}

	profiles := []string{"profile-1"}

	requestData := dto.NodeReqData{
		Flows:    nil,
		Rules:    rules,
		Profiles: profiles,
	}

	rulesDataMap := []map[string]interface{}{
		{"id": "rule-1", "label": "Test Rule"},
	}

	rulesJsonData, _ := json.Marshal(rulesDataMap)

	profileName := "profile-1"
	profilesJsonObject := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	profilesJsonData, _ := json.Marshal(profilesJsonObject)

	rulesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
	}

	profilesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(profilesJsonData)),
	}

	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules"
	})).Return(rulesResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules/rule-1"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
	}, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
	})).Return(profilesResponse, nil)
	client.Client = mockedHttpClient

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)

	if hedgeError != nil {
		t.Errorf("ExportDefs returned error, but expected success %v", hedgeError)
	}

	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", filePath)
	}
}

func TestDataMgmtService_ExportDefs_NoRules_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	mockedHttpClient := &svcmocks.MockHTTPClient{}

	flows := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"flow-1"},
		},
	}

	profiles := []string{"profile-1"}

	requestData := dto.NodeReqData{
		Flows:    flows,
		Rules:    nil,
		Profiles: profiles,
	}

	flowsDataMap := map[string]interface{}{
		"id": "flow-1", "label": "Test Flow",
	}

	flowJsonData, _ := json.Marshal(flowsDataMap)

	profileName := "profile-1"
	profilesJsonObject := dto.ProfileObject{
		Profile: dtos.DeviceProfile{
			DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
				Name: profileName,
			},
		},
	}
	profilesJsonData, _ := json.Marshal(profilesJsonObject)

	flowsResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	profilesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(profilesJsonData)),
	}

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "http://"+NATSPROXY+"/flow/flow-1")
	})).Return(flowsResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profileName
	})).Return(profilesResponse, nil)
	client.Client = mockedHttpClient

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)
	if hedgeError != nil {
		t.Errorf("ExportDefs returned error, but expected success %v", hedgeError)
	}

	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", filePath)
	}
}

func TestDataMgmtService_ExportDefs_NoProfiles_Success(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	mockedHttpClient := &svcmocks.MockHTTPClient{}

	flows := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"flow-1"},
		},
	}

	rules := []dto.ExportNodeData{
		{
			NodeID: "node-1",
			Host:   "host-1",
			Ids:    []string{"rule-1"},
		},
	}

	requestData := dto.NodeReqData{
		Flows:    flows,
		Rules:    rules,
		Profiles: nil,
	}

	flowsDataMap := map[string]interface{}{
		"id": "flow-1", "label": "Test Flow",
	}

	rulesDataMap := []map[string]interface{}{
		{"id": "rule-1", "label": "Test Rule"},
	}

	flowJsonData, _ := json.Marshal(flowsDataMap)

	rulesJsonData, _ := json.Marshal(rulesDataMap)

	flowsResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(flowJsonData)),
	}

	rulesResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(rulesJsonData)),
	}

	expectedRuleData := dto.RuleData{Id: "rule1", Sql: "Test Rule Sql Data"}
	expectedRuleDataJson, _ := json.Marshal(expectedRuleData)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "http://"+NATSPROXY+"/flow/flow-1")
	})).Return(flowsResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules"
	})).Return(rulesResponse, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.String() == "http://"+NATSPROXY+"/rules/rule-1"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedRuleDataJson)),
	}, nil)
	client.Client = mockedHttpClient

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	filePath, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)
	if hedgeError != nil {
		t.Errorf("ExportDefs returned error, but expected success %v", hedgeError)
	}

	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it is was not found", filePath)
	}
}

func TestDataMgmtService_ExportDefs_NoData_Fail(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	requestData := dto.NodeReqData{
		Flows:    nil,
		Rules:    nil,
		Profiles: nil,
	}

	_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	_, hedgeError := dataMgmtService.ExportDefs(requestData, baseTmpFolder)
	if hedgeError == nil {
		t.Errorf("ExportDefs returned success, but expected error")
	}

	expectedError := hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Failed to create index file"))
	if !reflect.DeepEqual(hedgeError, expectedError) {
		t.Errorf("Error recieved and expected error don't match. Recieved: %v, Expected: %v", hedgeError, expectedError)
	}
}

func TestDataMgmtService_ImportDefs(t *testing.T) {
	dataMgmtService := InitDataMgmtServiceTest()

	e := echo.New()

	response := &http.Response{
		StatusCode: http.StatusOK,
	}

	tests := []struct {
		name          string
		indexData     interface{}
		flowsData     []dto.Flow
		rulesData     []dto.Rule
		profilesData  []dto.ProfileObject
		filename      string
		expectError   bool
		expectedError error
	}{
		{
			name: "Valid data",
			indexData: dto.ExportImportIndex{
				Imports: dto.NodeData{
					ToNodeId: []string{"node2"},
					Flows:    []string{"flow1"},
					Rules:    []string{"rule1"},
					Profiles: []string{"profile1"},
				},
				FilePath: "test/index.json",
			},
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: false, Data: map[string]interface{}{"id": "flow1"}},
			},
			rulesData: []dto.Rule{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Name: []string{}, Data: dto.RuleData{Id: "rule1", Sql: "SELECT * FROM sensor_data WHERE value > 50", Actions: nil, Options: nil}},
			},
			profilesData: []dto.ProfileObject{
				{Profile: dtos.DeviceProfile{
					DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
						Name: "profile1",
					},
				},
				}},
			expectError: false,
			filename:    "test.zip",
		},
		{
			name:      "Invalid index data",
			indexData: "",
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: true, Data: nil},
			},
			rulesData:     []dto.Rule{},
			profilesData:  []dto.ProfileObject{},
			expectError:   true,
			filename:      "test.zip",
			expectedError: hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting index data from file  index.json: Error reading file index.json"),
		},
		{
			name:      "Invalid zip file",
			indexData: "",
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: true, Data: nil},
			},
			rulesData:     []dto.Rule{},
			profilesData:  []dto.ProfileObject{},
			expectError:   true,
			filename:      "test.txt",
			expectedError: hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "File is not in '.zip' format"),
		},
		{
			name: "Invalid NodeId List",
			indexData: dto.ExportImportIndex{
				Imports: dto.NodeData{
					ToNodeId: []string{},
					Flows:    []string{"flow1"},
					Rules:    []string{"rule1"},
					Profiles: []string{"profile1"},
				},
				FilePath: "test/index.json",
			},
			flowsData: []dto.Flow{
				{FromNodeId: "node1", ToNodeId: []string{"node2"}, Selected: true, Data: nil},
			},
			rulesData:     []dto.Rule{},
			profilesData:  []dto.ProfileObject{},
			expectError:   true,
			filename:      "test.zip",
			expectedError: hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "There are no Nodes to import the data into"),
		},
	}
	mockedHttpClient := &svcmocks.MockHTTPClient{}

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == "http://"+NATSPROXY+"/flow"
	})).Return(response, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == "http://"+KUIPER+"/rules"
	})).Return(response, nil)

	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == "http://"+NATSPROXY+"/rules"
	})).Return(response, nil)
	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == "http://"+KUIPER+"/rules/rule1"
	})).Return(response, nil)
	mockedHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.String() == "http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/profile1"
	})).Return(response, nil)
	client.Client = mockedHttpClient
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			zipBuffer, err := createTestZipFileForParseZipFile(tt.indexData, tt.flowsData, tt.rulesData, tt.profilesData)
			assert.NoError(t, err)

			part, err := writer.CreateFormFile("file", tt.filename)
			assert.NoError(t, err)

			_, err = io.Copy(part, zipBuffer)
			assert.NoError(t, err)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			MaxImportExportFileSizeBytes := int64(10 * 1024 * 1024) // 10MB limit
			c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, MaxImportExportFileSizeBytes)

			file, err := c.FormFile("file")
			assert.NoError(t, err)

			hedgeErr := dataMgmtService.ImportDefs(file)

			if tt.expectError {
				assert.Error(t, hedgeErr)
			} else {
				assert.NoError(t, hedgeErr)
			}
		})
	}
}
