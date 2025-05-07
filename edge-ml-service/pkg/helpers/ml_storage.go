/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hedge/edge-ml-service/pkg/dto/config"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
)

var (
	mkdirAllFunc = os.MkdirAll
	openFile     = os.Open
)

type ZipFileInfo struct {
	FilePath      string
	FilePathInZip string
}

type FileInZipInfo struct {
	Path string
	Size uint64
}

const (
	TRAINING_ZIP_FILENAME = "training_input.zip"
	MODEL_ZIP_FILENAME    = "hedge_export.zip"
)

type MLStorageInterface interface {
	GetMLStorage(
		baseLocalDirectory string,
		mlAlgorithm string,
		mlTrainingDataConfig string,
		lc logger.LoggingClient,
	) MLStorageInterface
	GetLocalTrainingDataBaseDir() string
	GetRemoteConfigFileName() string
	GeneratorRemoteTrainingDataZipFileName() string
	GeneratorRemoteConfigJsonFileName() string
	GetRemoteTrainingDataFileName() string
	GetTrainingDataFileName() string
	GetMLModelConfigFileName(isTrainingDataGeneration bool) string
	GetModelDir() string
	GetModelLocalDir() string
	GetModelLocalZipFileName() string
	GetModelIgnoreFileName() string
	GetBaseLocalDirectory() string
	AddModelIgnoreFile() error
	RemoveModelIgnoreFile() error
	InitializeFile(fileName string) error
	AddModelAndConfigFile(modelBytes []byte) error
	AddTrainingDataToFileWithFileName(data [][]string, headers []string, fileName string) error
	AddTrainingDataToFile(data [][]string, headers []string) error
	FileExists(filename string) bool
	GetTrainingInputZipFile() string
	GetModelZipFile() string
	CompressFolder(inputFolder string, zipFileName string) error
	CompressFiles(inputFiles []ZipFileInfo, zipFilePath string) error
	GetValidationLocalDir() string
}

var MLStorageInterfaceImpl MLStorageInterface

type MLStorage struct {
	BaseLocalDirectory string
	mlAlgorithm        string
	mlModelConfigName  string
	lc                 logger.LoggingClient
}

func NewMLStorage(
	baseLocalDirectory string,
	mlAlgorithm string,
	mlTrainingDataConfig string,
	lc logger.LoggingClient,
) MLStorageInterface {
	// For unit test case writing, we override the MLStorageInterfaceImpl
	if MLStorageInterfaceImpl == nil {
		MLStorageInterfaceImpl = &MLStorage{}
	}

	return MLStorageInterfaceImpl.GetMLStorage(
		baseLocalDirectory,
		mlAlgorithm,
		mlTrainingDataConfig,
		lc,
	)
}

func (m *MLStorage) GetMLStorage(
	baseLocalDirectory string,
	mlAlgorithm string,
	mlModelConfigName string,
	lc logger.LoggingClient,
) MLStorageInterface {
	mlStorage := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
		mlAlgorithm:        mlAlgorithm,
		mlModelConfigName:  mlModelConfigName,
		lc:                 lc,
	}
	return mlStorage
}

func (s *MLStorage) GetLocalTrainingDataBaseDir() string {
	return filepath.Join(s.BaseLocalDirectory, s.mlAlgorithm, s.mlModelConfigName, "data")
}

func (s *MLStorage) GetRemoteConfigFileName() string {
	return filepath.Join(s.mlAlgorithm, s.mlModelConfigName, "assets", "config.json")
}

// Since it is tricky to overwrite old file, we want to rename the training file every time we need to upload
func (s *MLStorage) GeneratorRemoteTrainingDataZipFileName() string {
	zipFileName := filepath.Join(
		s.mlAlgorithm,
		s.mlModelConfigName,
		fmt.Sprintf("data_%d.zip", time.Now().UnixMilli()/1000),
	)
	return zipFileName
}

// Since it is tricky to overwrite old file, we want to rename the training file every time we need to upload
func (s *MLStorage) GeneratorRemoteConfigJsonFileName() string {
	fileName := filepath.Join(
		s.mlAlgorithm,
		s.mlModelConfigName,
		fmt.Sprintf("config_%d.json", time.Now().UnixMilli()/1000),
	)
	return fileName
}

// For now only for GCP, we will change this to have zip file so this won't be applicable
func (s *MLStorage) GetRemoteTrainingDataFileName() string {
	trainingDataFileNameRemote := "training_data.csv"

	// The filename is here needs to be aligned with what is defined in python training code
	// remoteTrainingDataFile := s.mlAlgorithm + "/" + s.mlTrainingDataConfig+ "/" + jobExecutor.appConfig.TrainingDataDir + "/" + trainingDataFileNameRemote

	// Hardcoded just to get old GCP zip file code work
	remoteTrainingDataFile := filepath.Join(
		s.mlAlgorithm,
		s.mlModelConfigName,
		"training_data",
		trainingDataFileNameRemote,
	)
	return remoteTrainingDataFile
}

func (s *MLStorage) GetTrainingDataFileName() string {
	trainingDataFileName := filepath.Join(
		s.GetLocalTrainingDataBaseDir(),
		s.mlModelConfigName+".csv",
	)
	return trainingDataFileName
}

func (s *MLStorage) GetMLModelConfigFileName(isTrainingDataGeneration bool) string {
	mlModelConfigFileName := "config.json"
	// Training data config is kept at 2 different places during data-generation and then when model is trained
	// This is so that after model training, the training data config file might get overwritten
	if isTrainingDataGeneration {
		mlModelConfigFileName = filepath.Join(
			s.GetLocalTrainingDataBaseDir(),
			mlModelConfigFileName,
		)
	} else {
		mlModelConfigFileName = filepath.Join(s.GetModelLocalDir(), mlModelConfigFileName)
	}
	return mlModelConfigFileName
}

func (s *MLStorage) GetModelDir() string {
	return filepath.Join(s.mlAlgorithm, s.mlModelConfigName, "hedge_export")
}

func (s *MLStorage) GetModelLocalDir() string {
	return filepath.Join(s.BaseLocalDirectory, s.GetModelDir())
}

func (s *MLStorage) GetModelLocalZipFileName() string {
	// Replace by saved_model.h5 when time comes
	modelFile := filepath.Join(
		s.BaseLocalDirectory,
		s.mlAlgorithm,
		s.mlModelConfigName,
		MODEL_ZIP_FILENAME,
	)
	return modelFile
}

func (s *MLStorage) GetModelIgnoreFileName() string {
	// Replace by saved_model.h5 when time comes
	modeIgnoreFile := filepath.Join(s.GetModelLocalDir(), ".modelignore")
	return modeIgnoreFile
}

func (s *MLStorage) GetBaseLocalDirectory() string {
	return s.BaseLocalDirectory
}

func (s *MLStorage) AddModelIgnoreFile() error {
	// Replace by saved_model.h5 when time comes
	modeIgnoreFile := s.GetModelIgnoreFileName()
	if s.FileExists(modeIgnoreFile) {
		return nil
	}

	dir := filepath.Dir(modeIgnoreFile)
	if err := mkdirAllFunc(dir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(modeIgnoreFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return nil
}

func (s *MLStorage) RemoveModelIgnoreFile() error {
	// Replace by saved_model.h5 when time comes
	modelIgnoreFile := s.GetModelIgnoreFileName()
	if !s.FileExists(modelIgnoreFile) {
		return nil
	}
	err := os.Remove(modelIgnoreFile)
	return err
}

func (s *MLStorage) InitializeFile(fileName string) error {
	_, err := os.Stat(filepath.Dir(fileName))
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm)
		if err != nil {
			s.lc.Errorf("%v", err)
			return err
		}
	}
	if s.FileExists(fileName) {
		err := os.Remove(fileName)
		if err != nil {
			s.lc.Errorf("Error deleting training file: %s, error: %v", fileName, err)
			return err
		}
	}
	return nil
}

func (s *MLStorage) AddModelAndConfigFile(modelBytes []byte) error {
	dst := s.BaseLocalDirectory
	zipReader, err := zip.NewReader(bytes.NewReader(modelBytes), int64(len(modelBytes)))
	if err != nil {
		errMsg := fmt.Sprintf("failed to create zip reader: %s", err.Error())
		s.lc.Errorf(errMsg)
		return errors.New(errMsg)
	}

	for _, f := range zipReader.File {
		filePath := filepath.Join(dst, f.Name)
		s.lc.Infof("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			errMsg := fmt.Sprintf("invalid file path: %s", filePath)
			s.lc.Errorf(errMsg)
			return errors.New(errMsg)
		}
		if f.FileInfo().IsDir() {
			s.lc.Infof("creating directory...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := mkdirAllFunc(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

func (s *MLStorage) AddTrainingDataToFile(data [][]string, headers []string) error {
	fileName := s.GetTrainingDataFileName()
	return s.AddTrainingDataToFileWithFileName(data, headers, fileName)
}

func (s *MLStorage) AddTrainingDataToFileWithFileName(
	data [][]string,
	headers []string,
	fileName string,
) error {
	var file *os.File
	var writer *csv.Writer
	var err error

	if !s.FileExists(fileName) {
		file, err = os.Create(fileName)
		if err != nil {
			s.lc.Warnf("Error creating file :%s, error: %v/n", fileName, err)
			return err
		}
		writer = csv.NewWriter(file)
		err = writer.Write(headers)
		if err != nil {
			s.lc.Warnf("Error writing headers to file :%v, error: %v/n", headers, err)
			return err
		}
		err = writer.WriteAll(data)
		if err != nil {
			s.lc.Warnf("Error writing data to file :%v, error: %v/n", data, err)
			return err
		}
	} else {
		file, err = os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		writer = csv.NewWriter(file)
		err = writer.WriteAll(data)
		if err != nil {
			s.lc.Warnf("Error writing data to file :%v, error: %v/n", data, err)
			return err
		}
	}
	writer.Flush()
	file.Close()
	return nil
}

func (s *MLStorage) FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (s *MLStorage) GetTrainingInputZipFile() string {
	return filepath.Join(
		s.BaseLocalDirectory,
		s.mlAlgorithm,
		s.mlModelConfigName,
		TRAINING_ZIP_FILENAME,
	)
}

func (s *MLStorage) GetModelZipFile() string {
	return filepath.Join(
		s.BaseLocalDirectory,
		s.mlAlgorithm,
		s.mlModelConfigName,
		MODEL_ZIP_FILENAME,
	)
}

// Zip function
func (s *MLStorage) CompressFolder(inputFolder string, zipFileName string) error {

	var targetFilePaths []string

	// get filepaths in all folders
	err := filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if info != nil && info.IsDir() {
			return nil
		}
		// add all the file paths to slice
		targetFilePaths = append(targetFilePaths, path)
		return nil
	})

	if err != nil {
		s.lc.Errorf("Error traversing the directory: %v\n", err)
		return err
	}

	// zip file logic starts here
	ZipFile, err := os.Create(zipFileName)
	if err != nil {
		s.lc.Errorf("Error Creating the zip File: %v\n", err)
		return err
	}

	defer ZipFile.Close()

	zipWriter := zip.NewWriter(ZipFile)
	defer zipWriter.Close()

	for _, targetFilePath := range targetFilePaths {
		file, err := openFile(targetFilePath)
		if err != nil {
			s.lc.Errorf("Error opening the file: %s, Error: %s\n", targetFilePath, err.Error())
			return err
		}
		defer file.Close()

		// create path in zip
		newFileBasePath := strings.Replace(
			targetFilePath,
			s.BaseLocalDirectory+string(os.PathSeparator),
			"",
			1,
		)
		w, err := zipWriter.Create(newFileBasePath)
		if err != nil {
			s.lc.Errorf("Error creating the file: %s, Error: %s\n", targetFilePath, err.Error())
			return err
		}
		// write file to zip
		_, err = io.Copy(w, file)
		if err != nil {
			s.lc.Errorf("Error copying the file: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (s *MLStorage) CompressFiles(inputFiles []ZipFileInfo, zipFilePath string) error {
	// zip file logic starts here
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		s.lc.Errorf("Error creating zip file %v: %v", zipFilePath, err.Error())
		return err
	}

	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, inputFile := range inputFiles {
		file, err := os.Open(inputFile.FilePath)
		if err != nil {
			s.lc.Errorf("Error opening the file: %v, Error: %v", inputFile.FilePath, err.Error())
			return err
		}
		defer file.Close()

		w, err := zipWriter.Create(inputFile.FilePathInZip)
		if err != nil {
			s.lc.Errorf(
				"Error creating the file: %v, Error: %v",
				inputFile.FilePathInZip,
				err.Error(),
			)
			return err
		}
		// write file to zip
		_, err = io.Copy(w, file)
		if err != nil {
			s.lc.Errorf("Error copying the file: %v, Error: %v", inputFile.FilePath, err.Error())
			return err
		}
	}

	return nil
}

func (s *MLStorage) GetValidationLocalDir() string {
	return filepath.Join(s.BaseLocalDirectory, s.mlAlgorithm, s.mlModelConfigName, "validation")
}

// Reads MLAlgorithmDefn and MLModelConfig given a base directly
// Returns a map of mlModelName to MLConfig
func ReadMLConfigWithAlgo(
	trainingConfigAndModelBasePath string,
	logger logger.LoggingClient,
	algoNameFilter string,
) (map[string]config.MLConfig, error) {
	configFileToConfigMap := make(map[string]config.MLConfig)

	dirInfo, err := os.ReadDir(trainingConfigAndModelBasePath)

	if err != nil {
		return nil, err
	}

	for _, directory := range dirInfo {
		logger.Infof("directory: %s", directory.Name())
		if directory.IsDir() {
			// Another loop to go over the algorithm
			fullPath := filepath.Join(trainingConfigAndModelBasePath, directory.Name())
			algoDirInfo, err := os.ReadDir(fullPath)
			if err != nil {
				continue
			}
			if algoNameFilter != "" && directory.Name() != algoNameFilter {
				continue
			}
			for _, trgDir := range algoDirInfo {
				// ignore the dir if we find .modelignore
				modelignoreFile := filepath.Join(
					fullPath,
					trgDir.Name(),
					"hedge_export",
					".modelignore",
				)
				if _, err := os.Stat(modelignoreFile); err == nil {
					logger.Infof("Ignoring the model: %s, it was undeployed", trgDir.Name())
					continue
				}

				// Read this config
				configFile, err := os.Open(
					filepath.Join(fullPath, trgDir.Name(), "hedge_export", "assets", "config.json"),
				)
				if err != nil {
					logger.Errorf("Error reading ml_model configuration directory, Error:%v", err)
					continue
				}
				configBytes, err := io.ReadAll(configFile)
				if err != nil {
					logger.Errorf("Error reading ml_model configuration: %s", err.Error())
				}
				configFile.Close()
				var mlConfig config.MLConfig
				err = json.Unmarshal(configBytes, &mlConfig)
				if err != nil {
					logger.Error(
						"%s",
						fmt.Errorf(
							"error reading/unmarshalling training config directory, Error:%v",
							err,
						),
					)
					continue
				}
				if mlConfig.MLModelConfig.Name == "" {
					continue
				}

				mlConfig.MLModelConfig.LocalModelStorageDir = filepath.Join(
					trainingConfigAndModelBasePath,
					mlConfig.MLModelConfig.Name,
					"hedge_export",
				)

				configFileToConfigMap[mlConfig.MLModelConfig.Name] = mlConfig
			}
		}

	}
	return configFileToConfigMap, nil
}

func ReadZipFiles(filePath string) ([]FileInZipInfo, error) {
	var zipFiles []FileInZipInfo

	// Read the ZIP file
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return zipFiles, err
	}
	defer zipReader.Close()

	// Iterate through the files in the ZIP archive
	for _, file := range zipReader.File {
		zipFileInfo := FileInZipInfo{
			Path: file.Name,
			Size: file.UncompressedSize64,
		}
		zipFiles = append(zipFiles, zipFileInfo)
	}

	return zipFiles, nil
}

func SaveFile(filePath string, data []byte) error {
	// create dir if not exist
	err := mkdirAllFunc(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}

	file, err := os.Create(
		filePath,
	) // will replace the file content in case the file already exists
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func SaveMultipartFile(srcFile multipart.File, dst string) error {
	// create dir if not exist
	err := mkdirAllFunc(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	// Copy the uploaded file to the destination file
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	err = dstFile.Sync()
	if err != nil {
		return err
	}
	return nil
}

func ExtractZipFile(zipFilePath string, inZipPath string, dstPath string) error {
	zipReader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if file.Name == inZipPath {
			// Open the file inside the zip
			srcFile, err := file.Open()
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
			err = dstFile.Sync()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("file %s not found in zip %s", inZipPath, zipFilePath)
}

func ReadFileData(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file's content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func CopyFile(src string, dst string) error {
	// create dir if not exist
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	err = mkdirAllFunc(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		return err
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	err = dstFile.Sync()
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// changeOwnership changes the owner of a directory and its contents if UID/GID exist
func ChangeFileOwnership(basePath, path string, uid, gid int) error {

	// Apply ownership recursively
	return filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %v", p, err)
		}
		chownErr := os.Chown(p, uid, gid)
		if chownErr != nil {
			return fmt.Errorf("failed to change ownership for %s: %v", p, chownErr)
		}
		return nil
	})
}

// checkIDExists verifies if a UID or GID exists on the system
/*func checkIDExists(id int, isUser bool) bool {
	var cmd *exec.Cmd
	if isUser {
		cmd = exec.Command("getent", "passwd", strconv.Itoa(id))
	} else {
		cmd = exec.Command("getent", "group", strconv.Itoa(id))
	}

	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) != ""
}*/
