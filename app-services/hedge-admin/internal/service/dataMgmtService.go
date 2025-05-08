package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hedge/app-services/hedge-admin/internal/config"
	"hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/labstack/echo/v4"
	"golang.org/x/exp/slices"
)

const (
	NATSPROXY             = "nats-proxy:48200"
	NODERED               = "hedge-node-red:1880"
	KUIPER                = "edgex-kuiper:59720"
	DEVEXT                = "hedge-device-extensions:48097"
	CONTENTS_FOLDER       = "/contents/"
	TIME_FORMAT           = "20060102150405"
	EXPORT_DATA_FILE_NAME = "exported_data"
	INDEX_FILE_NAME       = "index.json"
	FLOWS_FILE_NAME       = "flows"
	RULES_FILE_NAME       = "rules"
	PROFILES_FILE_NAME    = "profiles"
)

type DataMgmtService struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
}

func NewDataMgmtService(
	service interfaces.ApplicationService,
	appConfig *config.AppConfig,
) *DataMgmtService {
	dataMgmtService := new(DataMgmtService)
	dataMgmtService.service = service
	dataMgmtService.appConfig = appConfig
	setHttpClient()
	return dataMgmtService
}

func setHttpClient() {
	if client.Client == nil {
		client.Client = &http.Client{}
	}
}

func (d DataMgmtService) ExportDefs(
	requestData dto.NodeReqData,
	baseDir string,
) (string, hedgeErrors.HedgeError) {
	exportData := d.ExportData(requestData)
	if len(exportData.FlowsData.Errors) > 0 &&
		len(exportData.RulesData.Errors) > 0 &&
		len(exportData.ProfilesData.Errors) > 0 {

		errorMessage := buildExportErrorMessage(exportData)
		d.service.LoggingClient().Errorf(errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	hedgeError := d.createIndexFile(baseDir, &exportData)
	if hedgeError != nil {
		return "", hedgeError
	}
	if len(exportData.FlowsData.Errors) > 0 &&
		len(exportData.RulesData.Errors) > 0 &&
		len(exportData.ProfilesData.Errors) > 0 {

		errorMessage := buildExportErrorMessage(exportData)
		d.service.LoggingClient().Errorf(errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	d.createJsonFiles(baseDir, &exportData)
	if len(exportData.FlowsData.Errors) > 0 &&
		len(exportData.RulesData.Errors) > 0 &&
		len(exportData.ProfilesData.Errors) > 0 {
		errorMessage := buildExportErrorMessage(exportData)
		d.service.LoggingClient().Errorf(errorMessage)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}

	zipPath, hedgeError := d.createZipFile(baseDir, exportData)
	if hedgeError != nil {
		d.service.LoggingClient().Errorf(hedgeError.Error())
		return "", hedgeError
	}

	return zipPath, nil
}

func buildExportErrorMessage(exportData dto.ExportImportData) string {
	errorMessage := "Export failed with the following errors:\n"
	var flowErrors strings.Builder
	if len(exportData.FlowsData.Errors) > 0 {
		for _, nodeError := range exportData.FlowsData.Errors {
			flowErrors.WriteString(fmt.Sprintf("NodeID: %s\n", nodeError.NodeID))
			for _, err := range nodeError.Errors {
				flowErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Flows:\n" + flowErrors.String() + "\n"
	}

	var ruleErrors strings.Builder
	if len(exportData.RulesData.Errors) > 0 {
		for _, nodeError := range exportData.RulesData.Errors {
			ruleErrors.WriteString(fmt.Sprintf("NodeID: %s\n", nodeError.NodeID))
			for _, err := range nodeError.Errors {
				ruleErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Rules:\n" + ruleErrors.String() + "\n"
	}

	var profileErrors strings.Builder
	if len(exportData.ProfilesData.Errors) > 0 {
		for _, profileError := range exportData.ProfilesData.Errors {
			profileErrors.WriteString(fmt.Sprintf("ProfileID: %s\n", profileError.ProfileID))
			for _, err := range profileError.Errors {
				profileErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Profiles:\n" + profileErrors.String() + "\n"
	}

	return errorMessage
}

func (d DataMgmtService) ExportData(requestData dto.NodeReqData) dto.ExportImportData {
	var exportData dto.ExportImportData
	if len(requestData.Flows) > 0 {
		err := d.exportFlows(requestData.Flows, &exportData.FlowsData)
		if err != nil {
			exportData.FlowsData.Errors = append(exportData.FlowsData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{err},
			})
		}
	}

	if len(requestData.Rules) > 0 {
		err := d.exportRules(requestData.Rules, &exportData.RulesData)
		if err != nil {
			exportData.RulesData.Errors = append(exportData.RulesData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{err},
			})
		}
	}

	if len(requestData.Profiles) > 0 {
		err := d.exportProfiles(
			requestData.Profiles,
			&exportData.ProfilesData,
		)
		if err != nil {
			exportData.ProfilesData.Errors = append(exportData.ProfilesData.Errors, dto.ProfileError{
				Errors: []hedgeErrors.HedgeError{err},
			})
		}
	}
	return exportData
}

func (d DataMgmtService) createJsonFiles(baseDir string, exportData *dto.ExportImportData) {
	if len(exportData.FlowsData.Errors) == 0 && exportData.FlowsData.Flows != nil {
		fileName, hedgeError := writeFlowsJsonFile(d, baseDir, exportData.FlowsData.Flows)
		if hedgeError != nil {
			exportData.FlowsData.Errors = append(exportData.FlowsData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
		exportData.FlowsData.FilePath = fileName
	}
	if len(exportData.RulesData.Errors) == 0 && exportData.RulesData.Rules != nil {
		fileName, hedgeError := writeRulesJsonFile(d, baseDir, exportData.RulesData.Rules)
		if hedgeError != nil {
			exportData.RulesData.Errors = append(exportData.RulesData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
		exportData.RulesData.FilePath = fileName
	}

	if len(exportData.ProfilesData.Errors) == 0 && exportData.ProfilesData.Profiles != nil {
		fileName, hedgeError := writeProfilesJsonFile(d, baseDir, exportData.ProfilesData.Profiles)
		if hedgeError != nil {
			exportData.ProfilesData.Errors = append(exportData.ProfilesData.Errors, dto.ProfileError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
		exportData.ProfilesData.FilePath = fileName
	}
}

func (d DataMgmtService) createListOfFilesPath(
	exportData dto.ExportImportData,
) ([]string, hedgeErrors.HedgeError) {
	var filesPath []string
	if len(exportData.FlowsData.Errors) == 0 && exportData.FlowsData.FilePath != "" {
		filesPath = append(filesPath, exportData.FlowsData.FilePath)
	}
	if len(exportData.RulesData.Errors) == 0 && exportData.RulesData.FilePath != "" {
		filesPath = append(filesPath, exportData.RulesData.FilePath)
	}
	if len(exportData.ProfilesData.Errors) == 0 && exportData.ProfilesData.FilePath != "" {
		filesPath = append(filesPath, exportData.ProfilesData.FilePath)
	}
	if exportData.IndexData.FilePath != "" {
		filesPath = append(filesPath, exportData.IndexData.FilePath)
	}
	if len(filesPath) == 0 {
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("There is no data to export"),
		)
	}
	return filesPath, nil
}

func CheckZipFile(d DataMgmtService, fileName string) hedgeErrors.HedgeError {
	if filepath.Ext(fileName) != ".zip" {
		d.service.LoggingClient().Error("File is not in '.zip' format")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"File should be in zip format",
		)
	}

	return nil
}

func (d DataMgmtService) ParseZipFile(
	file *multipart.FileHeader,
) (dto.ExportImportData, hedgeErrors.HedgeError) {
	d.service.LoggingClient().Info("About to unzip uploaded file")

	errorMessage := "Error parsing zip file"

	files, err := file.Open()
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening uploaded file: %v", err)
		return dto.ExportImportData{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			err.Error(),
		)
	}
	defer files.Close()

	archive, err := zip.NewReader(files, file.Size)
	if err != nil {
		d.service.LoggingClient().Errorf("Error unzipping uploaded file: %v", err)
		return dto.ExportImportData{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}
	d.service.LoggingClient().Info("zip file was read successfully")

	var importData dto.ExportImportData
	indexFileFound := false
	for _, f := range archive.File {
		hedgeError := checkJsonFile(d, f.Name)
		if hedgeError != nil {
			d.service.LoggingClient().Errorf("%v", hedgeError)
			continue
		}

		switch {
		case f.Name == INDEX_FILE_NAME:
			indexData, hedgeError := d.getIndexData(f)
			if hedgeError != nil {
				return dto.ExportImportData{}, hedgeError
			}
			importData.IndexData = indexData
			indexFileFound = true
		case strings.Contains(f.Name, FLOWS_FILE_NAME):
			flowsData, hedgeError := d.getFlowsData(f)
			if hedgeError != nil {
				importData.FlowsData.Errors = append(importData.FlowsData.Errors, dto.NodeError{
					Errors: []hedgeErrors.HedgeError{hedgeError},
				})
				continue
			}
			importData.FlowsData.Flows = flowsData
		case strings.Contains(f.Name, RULES_FILE_NAME):
			rulesData, hedgeError := d.getRulesData(f)
			if hedgeError != nil {
				importData.RulesData.Errors = append(importData.RulesData.Errors, dto.NodeError{
					Errors: []hedgeErrors.HedgeError{hedgeError},
				})
				continue
			}
			importData.RulesData.Rules = rulesData
		case strings.Contains(f.Name, PROFILES_FILE_NAME):
			profilesData, hedgeError := d.getProfilesData(f)
			if hedgeError != nil {
				importData.ProfilesData.Errors = append(importData.ProfilesData.Errors, dto.ProfileError{
					Errors: []hedgeErrors.HedgeError{hedgeError},
				})
				continue
			}
			importData.ProfilesData.Profiles = profilesData
		}
	}

	if !indexFileFound {
		d.service.LoggingClient().Errorf("Index file not found in the zip")
		return dto.ExportImportData{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"Index file not found in the zip",
		)
	}

	return importData, nil

}

func (d DataMgmtService) createIndexFile(
	baseDir string,
	exportData *dto.ExportImportData,
) hedgeErrors.HedgeError {
	if exportData == nil {
		d.service.LoggingClient().Errorf("error create index file due exportData is nil")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to create index file",
		)
	}

	if exportData.FlowsData.FlowsIndexData == nil &&
		exportData.RulesData.RulesIndexData == nil &&
		exportData.ProfilesData.ProfilesIndexData == nil {
		d.service.LoggingClient().
			Errorf("error create index file due flows, rules and profiles index data is empty")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("Failed to create index file"),
		)
	}
	fileName := filepath.Join(baseDir, INDEX_FILE_NAME)
	indexFile, err := os.Create(fileName)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating file %s: %v", INDEX_FILE_NAME, err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Error creating index file",
		)

	}
	defer indexFile.Close()

	if len(exportData.FlowsData.Errors) == 0 && exportData.FlowsData.FlowsIndexData != nil {
		if hedgeError := d.writeToIndexFile(indexFile, FLOWS_FILE_NAME, exportData.FlowsData.FlowsIndexData); hedgeError != nil {
			d.service.LoggingClient().Error(hedgeError.Error())
			exportData.FlowsData.Errors = append(exportData.FlowsData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
	}

	if len(exportData.RulesData.Errors) == 0 && exportData.RulesData.RulesIndexData != nil {
		if hedgeError := d.writeToIndexFile(indexFile, RULES_FILE_NAME, exportData.RulesData.RulesIndexData); hedgeError != nil {
			d.service.LoggingClient().Error(hedgeError.Error())
			exportData.RulesData.Errors = append(exportData.RulesData.Errors, dto.NodeError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
	}

	if len(exportData.ProfilesData.Errors) == 0 && exportData.ProfilesData.ProfilesIndexData != nil {
		if hedgeError := d.writeToIndexFile(indexFile, PROFILES_FILE_NAME, exportData.ProfilesData.ProfilesIndexData); hedgeError != nil {
			d.service.LoggingClient().Error(hedgeError.Error())
			exportData.ProfilesData.Errors = append(exportData.ProfilesData.Errors, dto.ProfileError{
				Errors: []hedgeErrors.HedgeError{hedgeError},
			})
		}
	}
	exportData.IndexData.FilePath = fileName
	return nil
}

func (d DataMgmtService) writeToIndexFile(
	indexFile *os.File,
	key string,
	value interface{},
) hedgeErrors.HedgeError {
	d.service.LoggingClient().Infof("Writing into index file the %s", key)
	errorMessage := "Error writing to index file"
	file, err := os.OpenFile(indexFile.Name(), os.O_RDWR, 0644)
	if err != nil {
		d.service.LoggingClient().Errorf("failed to open file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	defer file.Close()

	// Decode JSON content into a map
	var jsonData map[string]interface{}
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&jsonData); err != nil {
		// If the error is due to an empty file, initialize an empty map
		if err.Error() == "EOF" {
			jsonData = make(map[string]interface{})
		} else {
			d.service.LoggingClient().Errorf("failed to decode JSON: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	// Add or update the key/value pair
	jsonData[key] = value

	// Seek to the beginning of the file and truncate it
	if err := file.Truncate(0); err != nil {
		d.service.LoggingClient().Errorf("failed to truncate file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	if _, err := file.Seek(0, 0); err != nil {
		d.service.LoggingClient().Errorf("failed to seek file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	// Create a new encoder and encode the updated map into the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "")
	if err := encoder.Encode(jsonData); err != nil {
		d.service.LoggingClient().Errorf("failed to encode JSON: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return nil
}

func (d DataMgmtService) createZipFile(
	baseDir string,
	exportData dto.ExportImportData,
) (string, hedgeErrors.HedgeError) {
	filesPath, hedgeError := d.createListOfFilesPath(exportData)
	defer d.removeJsonFiles(filesPath)

	if hedgeError != nil {
		d.service.LoggingClient().Errorf(hedgeError.Error())
		return "", hedgeError
	}

	errorMessage := "Error creating zip file"
	timestamp := time.Now().Format(TIME_FORMAT) // YYYYMMDDHHMMSS
	zipPath := filepath.Join(baseDir, fmt.Sprintf("%s_%s.zip", EXPORT_DATA_FILE_NAME, timestamp))
	zipFile, err := os.Create(zipPath)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating the file: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer zipFile.Close()

	d.service.LoggingClient().Infof("created a zip file: %v, %v", zipPath, err)
	hedgeError = d.addFilesToZip(zipFile, filesPath)
	if hedgeError != nil {
		zipFile.Close()
		err = d.removeZipFile(zipPath)
		if err != nil {
			d.service.LoggingClient().
				Errorf("Failed to remove zip file after error in createZipFile: %v, %v", zipPath, err)
		}
		return "", hedgeError
	}
	return zipPath, nil
}

func (d DataMgmtService) addFilesToZip(
	zipFile *os.File,
	filesPath []string,
) hedgeErrors.HedgeError {
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	for _, filePath := range filesPath {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			d.service.LoggingClient().Errorf("Error reading the file: %v", err)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf("%s: file not found", "Error creating zip file"),
			)
		}
		err := d.addFileToZip(zipWriter, filePath)
		if err != nil {
			d.service.LoggingClient().Errorf("Error adding file to archive: %v", err)
			return err
		}
	}
	return nil
}

func (d DataMgmtService) addFileToZip(
	zipWriter *zip.Writer,
	filePath string,
) hedgeErrors.HedgeError {
	errorMessage := "Error adding file to zip file"

	file, err := os.Open(filePath)
	if err != nil {
		d.service.LoggingClient().Errorf("Error reading the file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer file.Close()

	wr, err := zipWriter.Create(filepath.Base(filePath))
	if err != nil {
		d.service.LoggingClient().Errorf("Error create the file in the zip: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	_, err = io.Copy(wr, file)
	if err != nil {
		d.service.LoggingClient().Errorf("Error copy the file to zip: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return nil
}

func (d DataMgmtService) removeJsonFiles(filesPath []string) hedgeErrors.HedgeError {
	for _, jsonFile := range filesPath {
		if _, err := os.Stat(jsonFile); !os.IsNotExist(err) {
			err := os.Remove(jsonFile)
			if err != nil {
				d.service.LoggingClient().Errorf("Error deleting JSON file %v: %v", jsonFile, err)
				return hedgeErrors.NewCommonHedgeError(
					hedgeErrors.ErrorTypeServerError,
					"Error deleting JSON file",
				)
			}
		}
	}
	return nil
}

func (d DataMgmtService) StreamZipFile(c echo.Context, zipPath string) hedgeErrors.HedgeError {
	errorMessage := "Error streaming zip file"

	zipFile, err := os.Open(zipPath)
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening the zip file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer d.removeZipFile(zipPath)
	defer zipFile.Close()

	hedgeError := d.checkZipFileSize(zipFile)
	if hedgeError != nil {
		return hedgeError
	}

	err = c.Attachment(zipPath, filepath.Base(zipPath))
	if err != nil {
		d.service.LoggingClient().Errorf("Error streaming the zip file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return nil
}

func (d DataMgmtService) removeZipFile(zipPath string) hedgeErrors.HedgeError {
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		err := os.Remove(zipPath)
		if err != nil {
			d.service.LoggingClient().Errorf("Error deleting the zip file: %v", err)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				"Error streaming zip file",
			)
		}
	}
	return nil
}
func (d DataMgmtService) checkZipFileSize(zipFile *os.File) hedgeErrors.HedgeError {
	fileInfo, err := zipFile.Stat()
	if err != nil {
		d.service.LoggingClient().Errorf("Error getting the file info: %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"Could not get file info",
		)
	}

	d.service.LoggingClient().Infof("max file size is %d", d.appConfig.MaxImportExportFileSizeMB)
	MaxImportExportFileSizeBytes := int64(d.appConfig.MaxImportExportFileSizeMB) * 1024 * 1024
	if fileInfo.Size() > MaxImportExportFileSizeBytes {
		d.service.LoggingClient().Errorf("File size exceeds the limit: %v bytes", fileInfo.Size())
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"File size exceeds the allowed limit",
		)
	}
	return nil
}

func (d DataMgmtService) ImportDefs(file *multipart.FileHeader) hedgeErrors.HedgeError {
	hedgeError := CheckZipFile(d, file.Filename)
	if hedgeError != nil {
		return hedgeError
	}

	importData, hedgeError := d.ParseZipFile(file)
	if hedgeError != nil {
		return hedgeError
	}
	if len(importData.IndexData.Imports.ToNodeId) == 0 {
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"There are no Nodes to import the data into",
		)
	}

	hedgeError = d.PrepareImportData(&importData)
	if hedgeError != nil {
		return hedgeError
	}
	d.ImportData(&importData)

	if len(importData.FlowsData.Errors) > 0 ||
		len(importData.RulesData.Errors) > 0 ||
		len(importData.ProfilesData.Errors) > 0 {

		errorMessage := buildImportErrorMessage(importData)
		d.service.LoggingClient().Errorf(errorMessage)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errorMessage,
		)
	}
	return nil

}

func buildImportErrorMessage(importData dto.ExportImportData) string {
	errorMessage := "Import failed with the following errors:\n"
	var flowErrors strings.Builder
	if len(importData.FlowsData.Errors) > 0 {
		for _, nodeError := range importData.FlowsData.Errors {
			flowErrors.WriteString(fmt.Sprintf("NodeID: %s\n", nodeError.NodeID))
			for _, err := range nodeError.Errors {
				flowErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Flows:\n" + flowErrors.String() + "\n"
	}

	var ruleErrors strings.Builder
	if len(importData.RulesData.Errors) > 0 {
		for _, nodeError := range importData.RulesData.Errors {
			ruleErrors.WriteString(fmt.Sprintf("NodeID: %s\n", nodeError.NodeID))
			for _, err := range nodeError.Errors {
				ruleErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Rules:\n" + ruleErrors.String() + "\n"
	}

	var profileErrors strings.Builder
	if len(importData.ProfilesData.Errors) > 0 {
		for _, profileError := range importData.ProfilesData.Errors {
			profileErrors.WriteString(fmt.Sprintf("ProfileID: %s\n", profileError.ProfileID))
			for _, err := range profileError.Errors {
				profileErrors.WriteString(fmt.Sprintf("  - %s: %s\n", err.ErrorType(), err.Message()))
			}
		}
		errorMessage += "Profiles:\n" + profileErrors.String() + "\n"
	}
	return errorMessage
}

func (d DataMgmtService) PrepareImportData(
	importData *dto.ExportImportData,
) hedgeErrors.HedgeError {
	if importData == nil {
		d.service.LoggingClient().Errorf("Import failed for due importData empty")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to Import all requested data",
		)
	}
	d.updateImportFlowData(importData)
	d.updateImportRulesData(importData)
	return nil
}

func (d DataMgmtService) updateImportFlowData(importData *dto.ExportImportData) {
	selectedFlows := []dto.Flow{}
	countFlows := len(importData.IndexData.Imports.Flows)
	for _, flow := range importData.FlowsData.Flows {
		if slices.Contains(importData.IndexData.Imports.Flows, fmt.Sprint(flow.Data["id"])) {
			flow.ToNodeId = importData.IndexData.Imports.ToNodeId
			if !flow.Selected {
				flow.Selected = true
				selectedFlows = append(selectedFlows, flow)
			}
			countFlows--
			if countFlows == 0 {
				break
			}
		}
	}
	importData.FlowsData.Flows = selectedFlows
}

func (d DataMgmtService) updateImportRulesData(importData *dto.ExportImportData) {
	selectedRules := []dto.Rule{}
	for _, rule := range importData.RulesData.Rules {
		if rule.Data.Id != "" && slices.Contains(importData.IndexData.Imports.Rules, rule.Data.Id) {
			rule.ToNodeId = importData.IndexData.Imports.ToNodeId
			selectedRules = append(selectedRules, rule)
		}
	}
	importData.RulesData.Rules = selectedRules
}

func (d DataMgmtService) ImportData(importData *dto.ExportImportData) {
	if importData != nil {
		d.importFlows(importData.FlowsData.Flows, &importData.FlowsData.Errors)
		d.importRules(importData.RulesData.Rules, &importData.RulesData.Errors)
		d.importProfiles(
			importData.ProfilesData.Profiles,
			importData.IndexData.Imports.Profiles,
			&importData.ProfilesData.Errors,
		)
	}
}

func (d DataMgmtService) getFlowsData(file *zip.File) ([]dto.Flow, hedgeErrors.HedgeError) {
	var flows []dto.Flow
	errorMessage := fmt.Sprintf("Error getting flows data from file %s", file.Name)
	jsonFile, err := file.Open()
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error opening file %s", errorMessage, file.Name),
		)
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	if err := decoder.Decode(&flows); err != nil {
		d.service.LoggingClient().Errorf("Error decoding file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error reading file %s", errorMessage, file.Name),
		)
	}

	d.service.LoggingClient().Debugf("Flows data content %v", flows)

	return flows, nil
}

func (d DataMgmtService) getRulesData(file *zip.File) ([]dto.Rule, hedgeErrors.HedgeError) {
	var rules []dto.Rule
	errorMessage := fmt.Sprintf("Error getting rules data from file %s", file.Name)
	jsonFile, err := file.Open()
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error opening file %s", errorMessage, file.Name),
		)
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	if err := decoder.Decode(&rules); err != nil {
		d.service.LoggingClient().Errorf("Error decoding file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error reading file %s", errorMessage, file.Name),
		)
	}

	d.service.LoggingClient().Debugf("Rules data content %v", rules)

	return rules, nil
}

func (d DataMgmtService) getProfilesData(
	file *zip.File,
) ([]dto.ProfileObject, hedgeErrors.HedgeError) {
	var profiles []dto.ProfileObject
	errorMessage := fmt.Sprintf("Error getting profiles data from file %s", file.Name)
	jsonFile, err := file.Open()
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error opening file %s", errorMessage, file.Name),
		)
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	if err := decoder.Decode(&profiles); err != nil {
		d.service.LoggingClient().Errorf("Error decoding file %s: %v", file.Name, err)
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error reading file %s", errorMessage, file.Name),
		)
	}

	d.service.LoggingClient().Debugf("Profiles data content %v", profiles)

	return profiles, nil
}

func (d DataMgmtService) getIndexData(
	file *zip.File,
) (dto.ExportImportIndex, hedgeErrors.HedgeError) {
	var indexData dto.ExportImportIndex
	errorMessage := fmt.Sprintf("Error getting index data from file  %s", file.Name)
	jsonFile, err := file.Open()
	if err != nil {
		d.service.LoggingClient().Errorf("Error opening %s: %v", INDEX_FILE_NAME, err)
		return dto.ExportImportIndex{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error opening file %s", errorMessage, file.Name),
		)
	}

	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)
	if err := decoder.Decode(&indexData); err != nil {
		d.service.LoggingClient().Errorf("Error decoding %s: %v", INDEX_FILE_NAME, err)
		return dto.ExportImportIndex{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error reading file %s", errorMessage, file.Name),
		)
	}

	d.service.LoggingClient().Debugf("Index data content %+v", indexData)
	d.service.LoggingClient().Info("extracting index file successfully")

	return indexData, nil
}

func checkJsonFile(d DataMgmtService, fileName string) hedgeErrors.HedgeError {
	if filepath.Ext(fileName) != ".json" {
		d.service.LoggingClient().Error("File is not in '.json' format")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			"File should be in json format",
		)
	}
	return nil
}

func (d DataMgmtService) importFlows(flows []dto.Flow, flowsError *[]dto.NodeError) {
	d.service.LoggingClient().Info("About to import flows")

	for _, flow := range flows {
		d.service.LoggingClient().Debugf("Flow data content %+v", flow.Data)
		for _, nodeId := range flow.ToNodeId {
			hedgeError := importFlowPerNodeId(d, flow, nodeId)
			if hedgeError != nil {
				var existingNodeError *dto.NodeError
				for i := range *flowsError {
					if (*flowsError)[i].NodeID == nodeId {
						existingNodeError = &(*flowsError)[i]
						break
					}
				}

				if existingNodeError != nil {
					existingNodeError.Errors = append(existingNodeError.Errors, hedgeError)
				} else {
					nodeError := dto.NodeError{
						NodeID: nodeId,
						Errors: []hedgeErrors.HedgeError{hedgeError},
					}
					*flowsError = append(*flowsError, nodeError)
				}
				continue
			}
		}
	}
}

func importFlowPerNodeId(d DataMgmtService, flow dto.Flow, node string) hedgeErrors.HedgeError {
	flowID := "unknown"
	if id, ok := flow.Data["id"].(string); ok && id != "" {
		flowID = id
	}
	errorMessage := fmt.Sprintf("Error importing flow with ID: %s", flowID)

	jsonByte, err := json.Marshal(flow.Data)
	if err != nil {
		d.service.LoggingClient().Errorf("Error trying to marshal flow data: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	payload := bytes.NewReader(jsonByte)
	if node == d.appConfig.NodeId {
		d.service.LoggingClient().Info("Sending request to node-red core")
		req, err := http.NewRequest(http.MethodPost, "http://"+NODERED+"/flow", payload)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		req.Header.Add("Content-Type", "application/json")
		d.service.LoggingClient().Debugf("import flows node-red req content: %+v", req)
		resp, err := client.Client.Do(req)
		if err != nil {
			d.service.LoggingClient().Errorf("Error sending request to node-red: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !statusOK {
			d.service.LoggingClient().
				Errorf("Error on sending a request to the endpoint %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: failed to add flows to node %s", errorMessage, node),
			)
		}
		d.service.LoggingClient().Debugf("import flows node-red resp content: %+v", resp)
	} else {
		d.service.LoggingClient().Info("Sending request to nats-proxy")
		req, err := http.NewRequest(http.MethodPost, "http://"+NATSPROXY+"/flow", payload)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-NODE-ID", node)
		req.Header.Add("X-Forwarded-For", NODERED)
		d.service.LoggingClient().Debugf("import flows nats-proxy req content: %+v", req)
		resp, err := client.Client.Do(req)
		if err != nil {
			d.service.LoggingClient().Errorf("Error sending request to nats-proxy: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !statusOK {
			d.service.LoggingClient().Errorf("Error on sending a request to the endpoint %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: failed to add flows to node %s", errorMessage, node))
		}
		d.service.LoggingClient().Info("sent request to nats-proxy successfully")
		d.service.LoggingClient().Debugf("import flows nats-proxy resp content: %+v", resp)
	}
	return nil
}

func (d DataMgmtService) importRules(rules []dto.Rule, rulesError *[]dto.NodeError) {
	d.service.LoggingClient().Info("About to import rules")

	for _, rule := range rules {
		for _, nodeId := range rule.ToNodeId {
			hedgeError := importRulePerNodeId(d, rule.Data, nodeId)
			if hedgeError != nil {
				var existingNodeError *dto.NodeError
				for i := range *rulesError {
					if (*rulesError)[i].NodeID == nodeId {
						existingNodeError = &(*rulesError)[i]
						break
					}
				}

				if existingNodeError != nil {
					existingNodeError.Errors = append(existingNodeError.Errors, hedgeError)
				} else {
					nodeError := dto.NodeError{
						NodeID: nodeId,
						Errors: []hedgeErrors.HedgeError{hedgeError},
					}
					*rulesError = append(*rulesError, nodeError)
				}
				continue
			}
		}
	}
}

func importRulePerNodeId(d DataMgmtService, data dto.RuleData, node string) hedgeErrors.HedgeError {
	errorMessage := "Error importing rule " + data.Id
	jsonByte, err := json.Marshal(data)
	if err != nil {
		d.service.LoggingClient().Errorf("Error trying to marshal rule data: %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s: fail to read rule data", errorMessage),
		)
	}
	payload := bytes.NewReader(jsonByte)
	if node == d.appConfig.NodeId {
		d.service.LoggingClient().Info("Sending request to kuiper")
		req, err := http.NewRequest(http.MethodPost, "http://"+KUIPER+"/rules", payload)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: failed to add rules to node %s", errorMessage, node),
			)
		}
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Client.Do(req)
		if err != nil {
			d.service.LoggingClient().
				Errorf("Error sending request to kuiper: %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: failed to add rules to node %s", errorMessage, node),
			)
		}
		statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
		d.service.LoggingClient().Errorf("resp.StatusCode: %v", resp.StatusCode)
		if !statusOK {
			d.service.LoggingClient().
				Errorf("Error on sending a request to the endpoint %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: failed to add rules to node %s. Possibly the rule already exist or there is an error on the rule engine", errorMessage, node),
			)
		}
		d.service.LoggingClient().Info("sent request to kuiper successfully")
		d.service.LoggingClient().Debugf("import rules kuiper resp content: %+v", resp)
	} else {
		d.service.LoggingClient().Info("Sending request to nats-proxy")
		req, err := http.NewRequest(http.MethodPost, "http://"+NATSPROXY+"/rules", payload)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: failed to add rules to node %s", errorMessage, node))
		}
		req.Header.Add("X-NODE-ID", node)
		req.Header.Add("X-Forwarded-For", KUIPER)
		resp, err := client.Client.Do(req)
		if err != nil {
			d.service.LoggingClient().Errorf("Error sending request to nats-proxy: %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: failed to add rules to node %s", errorMessage, node))
		}
		statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !statusOK {
			d.service.LoggingClient().Errorf("Error on sending a request to the endpoint %v: node %s", err, node)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: failed to add rules to node %s. Possibly the rule already exist or there is an error on the rule engine", errorMessage, node))
		}
		d.service.LoggingClient().Info("sent request to nats-proxy successfully")
		d.service.LoggingClient().Debugf("import rules nats-proxy resp content: %+v", resp)
	}
	return nil
}

func (d DataMgmtService) importProfiles(
	profiles []dto.ProfileObject,
	nodeProfiles []string,
	profilesError *[]dto.ProfileError,
) {
	d.service.LoggingClient().Info("About to import profiles")
	var hedgeError hedgeErrors.HedgeError
	for _, profile := range profiles {
		if slices.Contains(nodeProfiles, profile.Profile.DeviceProfileBasicInfo.Name) {
			hedgeError = importProfile(d, profile)
			if hedgeError != nil {
				var existingProfileError *dto.ProfileError
				for i := range *profilesError {
					if (*profilesError)[i].ProfileID == profile.Profile.Id {
						existingProfileError = &(*profilesError)[i]
						break
					}
				}

				if existingProfileError != nil {
					existingProfileError.Errors = append(existingProfileError.Errors, hedgeError)
				} else {
					profileError := dto.ProfileError{
						ProfileID: profile.Profile.Id,
						Errors:    []hedgeErrors.HedgeError{hedgeError},
					}
					*profilesError = append(*profilesError, profileError)
				}
				continue
			}
		} else {
			profileError := dto.ProfileError{
				Errors: []hedgeErrors.HedgeError{
					hedgeErrors.NewCommonHedgeError(
						hedgeErrors.ErrorTypeNotFound,
						fmt.Sprintf("Profile %s not found in node profiles", profile.Profile.DeviceProfileBasicInfo.Name),
					)},
			}
			*profilesError = append(*profilesError, profileError)
		}
	}
}

func importProfile(d DataMgmtService, profile dto.ProfileObject) hedgeErrors.HedgeError {
	errorMessage := "Error importing profiles " + profile.Profile.Id
	jsonByte, err := json.Marshal(profile)
	if err != nil {
		d.service.LoggingClient().Errorf("Error trying to marshal name data: %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s: failed to read profiles data", errorMessage),
		)
	}
	payload := bytes.NewReader(jsonByte)
	d.service.LoggingClient().Info("Sending request to device extension")
	req, err := http.NewRequest(
		http.MethodPost,
		"http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+profile.Profile.DeviceProfileBasicInfo.Name,
		payload,
	)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating new request: %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s: failed to add profiles", errorMessage),
		)
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Client.Do(req)
	if err != nil {
		d.service.LoggingClient().Errorf("Error sending request to device extensions: %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s: failed to add profiles", errorMessage),
		)
	}
	statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !statusOK {
		d.service.LoggingClient().Errorf("Error on sending a request to the endpoint %v", err)
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: failed to add profiles", errorMessage),
		)
	}
	d.service.LoggingClient().Debugf("import profiles resp content: %+v", resp)
	d.service.LoggingClient().Info("sent request to device extensions successfully")
	return nil
}

func (d DataMgmtService) exportFlows(
	exportNodesData []dto.ExportNodeData,
	flowsData *dto.ExportImportFlow,
) hedgeErrors.HedgeError {
	if flowsData == nil {
		d.service.LoggingClient().Errorf("Export failed for Flows because flows data is nil")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to export flows data",
		)
	}
	d.service.LoggingClient().Info("About to get flows from nodes")
	for _, exportNodeData := range exportNodesData {
		if exportNodeData.Ids == nil {
			d.service.LoggingClient().
				Error("Ids list should not be null, should contain a list of workflow ids")
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeBadRequest,
				"The node should contain a list of workflow ids",
			)
		}

		for _, flowId := range exportNodeData.Ids {
			flow, hedgeError := exportFlowPerNodeId(d, exportNodeData.NodeID, flowId)
			flowsData.Flows = append(flowsData.Flows, flow)
			if hedgeError != nil {
				var existingNodeError *dto.NodeError
				for i := range flowsData.Errors {
					if flowsData.Errors[i].NodeID == exportNodeData.NodeID {
						existingNodeError = &flowsData.Errors[i]
						break
					}
				}

				if existingNodeError != nil {
					existingNodeError.Errors = append(existingNodeError.Errors, hedgeError)
				} else {
					nodeError := dto.NodeError{
						NodeID: exportNodeData.NodeID,
						Errors: []hedgeErrors.HedgeError{hedgeError},
					}
					flowsData.Errors = append(flowsData.Errors, nodeError)
				}
				continue
			}
			flowIndex := createFlowEntryForIndexFile(flow.Data, exportNodeData.NodeID)
			flowsData.FlowsIndexData = append(flowsData.FlowsIndexData, flowIndex)

		}
	}
	return nil
}

func writeFlowsJsonFile(
	d DataMgmtService,
	baseDir string,
	flows []dto.Flow,
) (string, hedgeErrors.HedgeError) {
	errorMessage := "Error exporting flows"
	timestamp := time.Now().Format(TIME_FORMAT) // YYYYMMDDHHMMSS
	fileName := filepath.Join(baseDir, fmt.Sprintf("%s_%s.json", FLOWS_FILE_NAME, timestamp))
	outFile, err := os.Create(fileName)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating file flows.json: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error creating file %s", errorMessage, fileName),
		)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	err = encoder.Encode(flows)
	if err != nil {
		d.service.LoggingClient().Errorf("Error writing flows.json to file system: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error writing into file %s", errorMessage, fileName),
		)
	}
	d.service.LoggingClient().Info("Successfully wrote flows json file")
	return fileName, nil
}

func exportFlowPerNodeId(
	d DataMgmtService,
	nodeId string,
	flowId string,
) (dto.Flow, hedgeErrors.HedgeError) {
	var req *http.Request
	var err error
	errorMessage := "Error exporting flow " + flowId
	var bodyMap map[string]interface{}
	// check if getting data from core
	if nodeId == d.appConfig.NodeId {
		req, err = http.NewRequest(http.MethodGet, "http://"+NODERED+"/flow/"+flowId, nil)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v: node %s", err, nodeId)
			return dto.Flow{}, hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeBadRequest,
				fmt.Sprintf("%s from node %s", errorMessage, nodeId),
			)
		}
	} else {
		// else get data from node
		req, err = http.NewRequest(http.MethodGet, "http://"+NATSPROXY+"/flow/"+flowId, nil)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v: node %s", err, nodeId)
			return dto.Flow{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s from node %s", errorMessage, nodeId))
		}
		req.Header.Add("X-NODE-ID", nodeId)
		req.Header.Add("X-Forwarded-For", NODERED)
	}
	resp, err := client.Client.Do(req)
	if err != nil {
		d.service.LoggingClient().
			Errorf("Error sending request to: %v: from node %s", err, nodeId)
		return dto.Flow{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s from node %s", errorMessage, nodeId),
		)
	}
	d.service.LoggingClient().Debugf("export flows resp content: %+v", resp)
	bodyByte, err := io.ReadAll(resp.Body)
	if err := json.NewDecoder(bytes.NewBuffer(bodyByte)).Decode(&bodyMap); err != nil {
		d.service.LoggingClient().Errorf("Error parsig response: %v: from node %s", err, nodeId)
		return dto.Flow{}, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeBadRequest,
			fmt.Sprintf("%s from node %s", errorMessage, nodeId),
		)
	}

	flow := dto.Flow{
		FromNodeId: nodeId,
		Selected:   false,
		Data:       bodyMap,
	}
	return flow, nil
}

func createFlowEntryForIndexFile(
	bodyMap map[string]interface{},
	nodeId string,
) map[string]interface{} {
	entry := make(map[string]interface{})
	entry["id"] = bodyMap["id"]
	entry["fromNodeId"] = nodeId
	entry["label"] = bodyMap["label"]
	return entry
}

func (d DataMgmtService) exportRules(
	exportNodesData []dto.ExportNodeData,
	rulesData *dto.ExportImportRules,
) hedgeErrors.HedgeError {
	if rulesData == nil {
		d.service.LoggingClient().Errorf("Export failed for Rules because rules data is nil")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"Failed to export rules data",
		)
	}
	d.service.LoggingClient().Info("About to get rules from nodes")
	for _, exportNodeData := range exportNodesData {
		if exportNodeData.Ids == nil {
			d.service.LoggingClient().
				Error("Ids list should not be null, should contain a list of rules ids")
			return hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeBadRequest,
				"The node should contain a list of rules ids",
			)
		}
		var bodyMap []map[string]interface{}
		err := getRuleList(d, exportNodeData, &bodyMap)
		if err != nil {
			d.service.LoggingClient().Errorf("Error getting rules list: %v", err)
			return err
		}
		var rule dto.Rule
		rule.Data = dto.RuleData{}
		rule.FromNodeId = exportNodeData.NodeID
		for _, id := range exportNodeData.Ids {
			ruleData := dto.RuleData{}
			err = createRuleData(d, exportNodeData, bodyMap, &ruleData, id)
			if err != nil {
				d.service.LoggingClient().Error(err.Error())
				var existingNodeError *dto.NodeError
				for i := range rulesData.Errors {
					if rulesData.Errors[i].NodeID == exportNodeData.NodeID {
						existingNodeError = &rulesData.Errors[i]
						break
					}
				}

				if existingNodeError != nil {
					existingNodeError.Errors = append(existingNodeError.Errors, err)
				} else {
					nodeError := dto.NodeError{
						NodeID: exportNodeData.NodeID,
						Errors: []hedgeErrors.HedgeError{err},
					}
					rulesData.Errors = append(rulesData.Errors, nodeError)
				}
				continue
			}

			rule.Data = ruleData
			rulesData.Rules = append(rulesData.Rules, rule)
			ruleIndex := createRuleEntryForIndexFile(rule.Data.Id, exportNodeData.NodeID)
			rulesData.RulesIndexData = append(rulesData.RulesIndexData, ruleIndex)
		}
	}
	return nil
}

func writeRulesJsonFile(
	d DataMgmtService,
	baseDir string,
	rules []dto.Rule,
) (string, hedgeErrors.HedgeError) {
	errorMessage := "Error exporting rules"
	timestamp := time.Now().Format(TIME_FORMAT) // YYYYMMDDHHMMSS
	fileName := filepath.Join(baseDir, fmt.Sprintf("%s_%s.json", RULES_FILE_NAME, timestamp))
	outFile, err := os.Create(fileName)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating file rules.json: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error creating file %s", errorMessage, fileName),
		)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	err = encoder.Encode(rules)
	if err != nil {
		d.service.LoggingClient().Errorf("Error writing rules.json to file system: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error writing into file %s", errorMessage, fileName),
		)
	}
	d.service.LoggingClient().Info("Successfully wrote rules json file")
	return fileName, nil
}

func createRuleEntryForIndexFile(ruleId string, nodeId string) map[string]interface{} {
	var entry = make(map[string]interface{})
	entry["id"] = ruleId
	entry["fromNodeId"] = nodeId
	return entry
}

func getRuleList(
	d DataMgmtService,
	node dto.ExportNodeData,
	bodyMap *[]map[string]interface{},
) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error fetching rule list from node %s", node.NodeID)

	var req *http.Request
	var err error
	if node.NodeID == d.appConfig.NodeId {
		req, err = http.NewRequest(http.MethodGet, "http://"+KUIPER+"/rules", nil)
		if err != nil {
			d.service.LoggingClient().
				Errorf("Error creating new request: %v: node %s", err, node.NodeID)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		d.service.LoggingClient().Debugf("Rule list ekuiper req content: %+v", req)
	} else {
		req, err = http.NewRequest(http.MethodGet, "http://"+NATSPROXY+"/rules", nil)
		if err != nil {
			d.service.LoggingClient().Errorf("Error creating new request: %v: node %s", err, node.NodeID)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		req.Header.Add("X-NODE-ID", node.NodeID)
		req.Header.Add("X-Forwarded-For", KUIPER)
		d.service.LoggingClient().Debugf("Rule list natsproxy req content: %+v", req)
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		d.service.LoggingClient().Errorf("Error sending request to nats-proxy: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	d.service.LoggingClient().Debugf("Rule list resp content: %+v", resp)

	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		d.service.LoggingClient().Errorf("Error reading response body: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	if err := json.NewDecoder(bytes.NewBuffer(bodyByte)).Decode(&bodyMap); err != nil {
		d.service.LoggingClient().Errorf("Reading rule list from nodes failed: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	d.service.LoggingClient().Debugf("Rule list content: %+v", bodyMap)
	return nil
}

func createRuleData(
	d DataMgmtService,
	exportNodeData dto.ExportNodeData,
	bodyMap []map[string]interface{},
	ruleData *dto.RuleData,
	ruleId string,
) hedgeErrors.HedgeError {
	var req *http.Request
	var err error

	errorMessage := fmt.Sprintf("Error creating rule id %s data for node %s", ruleId, exportNodeData.NodeID)
	for _, data := range bodyMap {
		if data["id"] != ruleId {
			continue
		}
		if exportNodeData.NodeID == d.appConfig.NodeId {
			req, err = http.NewRequest(
				http.MethodGet,
				"http://"+KUIPER+"/rules/"+data["id"].(string),
				nil,
			)
			if err != nil {
				d.service.LoggingClient().Errorf("Error creating new request: %v", err)
				return hedgeErrors.NewCommonHedgeError(
					hedgeErrors.ErrorTypeBadRequest,
					errorMessage,
				)
			}
			d.service.LoggingClient().Debugf("Create rule data req kuiper content: %+v", req)
		} else {
			endpoint := "http://" + NATSPROXY + "/rules/" + data["id"].(string)
			d.service.LoggingClient().Debugf("About to get rule data by rule id from %s", endpoint)
			req, err = http.NewRequest(http.MethodGet, "http://"+NATSPROXY+"/rules/"+data["id"].(string), nil)
			if err != nil {
				d.service.LoggingClient().Errorf("Error creating new request: %v", err)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
			}
			req.Header.Add("X-NODE-ID", exportNodeData.NodeID)
			req.Header.Add("X-Forwarded-For", KUIPER)
			d.service.LoggingClient().Debugf("Create rule data req nats-proxy content: %+v", req)
		}

		resp, err := client.Client.Do(req)
		if err != nil {
			d.service.LoggingClient().Errorf("Error sending request to nats-proxy: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		d.service.LoggingClient().Debugf("Create rule data resp content: %+v", resp)

		bodyByte, err := io.ReadAll(resp.Body)
		if err != nil {
			d.service.LoggingClient().Errorf("Error reading response body: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}

		d.service.LoggingClient().Info("About to read rule data")
		if err := json.NewDecoder(bytes.NewBuffer(bodyByte)).Decode(&ruleData); err != nil {
			d.service.LoggingClient().Errorf(err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
		d.service.LoggingClient().Debugf("Create rule data content: %+v", ruleData)
	}

	return nil
}

func (d DataMgmtService) exportProfiles(
	profileNames []string,
	profilesData *dto.ExportImportProfiles,
) hedgeErrors.HedgeError {
	if profilesData == nil {
		d.service.LoggingClient().Errorf("Export failed for Profiles because profiles data is nil")
		return hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("Failed to export profiles data"),
		)
	}
	d.service.LoggingClient().Info("About to get profiles")
	var compProfile dto.ProfileObject
	var compProfiles []dto.ProfileObject

	for _, name := range profileNames {
		// Get extensions, down-sampling, etc
		err := getCompleteProfile(name, &compProfile, &compProfiles, d)

		if err != nil {
			d.service.LoggingClient().Errorf("Error getting profile: %s", name)
			var existingProfileError *dto.ProfileError
			for i := range profilesData.Errors {
				if profilesData.Errors[i].ProfileID == name {
					existingProfileError = &profilesData.Errors[i]
					break
				}
			}

			if existingProfileError != nil {
				existingProfileError.Errors = append(existingProfileError.Errors, err)
			} else {
				profileError := dto.ProfileError{
					ProfileID: name,
					Errors:    []hedgeErrors.HedgeError{err},
				}
				profilesData.Errors = append(profilesData.Errors, profileError)
			}
			continue
		}
	}
	profilesData.Profiles = compProfiles
	profilesData.ProfilesIndexData = profileNames
	return nil
}

func writeProfilesJsonFile(
	d DataMgmtService,
	baseDir string,
	compProfiles []dto.ProfileObject,
) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Error exporting profiles")
	timestamp := time.Now().Format(TIME_FORMAT) // YYYYMMDDHHMMSS
	fileName := filepath.Join(baseDir, fmt.Sprintf("%s_%s.json", PROFILES_FILE_NAME, timestamp))
	outFile, err := os.Create(fileName)
	defer outFile.Close()
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating file profiles.json: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error creating file %s", errorMessage, fileName),
		)

	}
	encoder := json.NewEncoder(outFile)

	// Write profiles to json file
	err = encoder.Encode(compProfiles)
	if err != nil {
		d.service.LoggingClient().Errorf("Error writing profiles.json to file system: %v", err)
		return "", hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("%s: Error writing into file %s", errorMessage, fileName),
		)
	}
	outFile.Close()
	d.service.LoggingClient().Info("Successfully wrote profile json file")
	return fileName, nil
}

func getCompleteProfile(
	name string,
	profile *dto.ProfileObject,
	profiles *[]dto.ProfileObject,
	d DataMgmtService,
) hedgeErrors.HedgeError {
	errorMessage := fmt.Sprintf("Error fetching complete profile %v", name)

	req, err := http.NewRequest(
		http.MethodGet,
		"http://"+DEVEXT+"/api/v3/deviceinfo/profile/name/"+name,
		nil,
	)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating new request: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		d.service.LoggingClient().Errorf("Error sending request to nats-proxy: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	bodyByte, err := io.ReadAll(resp.Body)
	if err := json.NewDecoder(bytes.NewBuffer(bodyByte)).Decode(&profile); err != nil {
		d.service.LoggingClient().Errorf(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	*profiles = append(*profiles, *profile)
	return nil
}
