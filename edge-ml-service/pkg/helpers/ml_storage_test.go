package helpers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMLStorage(t *testing.T) {
	storage := NewMLStorage(baseTmpFolder, "RandomForest", "training_data_config", u.AppService.LoggingClient())
	if storage == nil {
		t.Errorf("NewMLStorage returned nil, expected non-nil MLStorageInterface")
	}
}

func TestMLStorage_AddModelAndConfigFile(t *testing.T) {
	t.Run("AddModelAndConfigFile - Passed (Valid zip file)", func(t *testing.T) {
		testFiles := map[string]bool{
			"dummy.txt": true,
		}
		buf := createDummyZip(t, baseTmpFolder, testFiles)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mlStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mlStorage.AddModelAndConfigFile(buf.Bytes())
		assert.NoError(t, err)
		assert.FileExists(t, filepath.Join(baseTmpFolder, "dummy.txt"))
	})
	t.Run("AddModelAndConfigFile - Failed (Path traversal)", func(t *testing.T) {
		testFiles := map[string]bool{
			"maliciousfile.txt": false,
		}
		buf := createDummyZip(t, baseTmpFolder, testFiles)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mlStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mlStorage.AddModelAndConfigFile(buf.Bytes())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid file path")
	})
	t.Run("AddModelAndConfigFile - Failed (Invalid zip file)", func(t *testing.T) {
		invalidZipData := []byte("not a zip file")
		mlStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mlStorage.AddModelAndConfigFile(invalidZipData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "zip: not a valid zip file")
	})
	t.Run("AddModelAndConfigFile - Failed (Directory creation fails)", func(t *testing.T) {
		originalMkdirAll := mkdirAllFunc
		mkdirAllFunc = func(path string, perm os.FileMode) error {
			return fmt.Errorf("simulated mkdir creation error")
		}
		testFiles := map[string]bool{
			"dummy.txt": true,
		}
		buf := createDummyZip(t, baseTmpFolder, testFiles)
		t.Cleanup(func() {
			mkdirAllFunc = originalMkdirAll
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mlStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mlStorage.AddModelAndConfigFile(buf.Bytes())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "simulated mkdir creation error", "Error message does not indicate file creation failure")
	})
}

func TestMLStorage_AddModelIgnoreFile(t *testing.T) {
	modeIgnoreFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "hedge_export", ".modelignore")
	t.Run("AddModelIgnoreFile - Passed", func(t *testing.T) {
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mockMLStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			mlAlgorithm:        testMlAlgorithmName,
			mlModelConfigName:  testMlModelConfigName,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mockMLStorage.AddModelIgnoreFile()
		assert.NoError(t, err, "Expected no error for successful file creation")
		assert.FileExists(t, modeIgnoreFileName, "Expected the model ignore file to be created")
	})
	t.Run("AddModelIgnoreFile - Failed (File already exists)", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Dir(modeIgnoreFileName), os.ModePerm)
		modeIgnoreFile, _ := os.Create(modeIgnoreFileName)

		t.Cleanup(func() {
			modeIgnoreFile.Close()
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mockMLStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			mlAlgorithm:        testMlAlgorithmName,
			mlModelConfigName:  testMlModelConfigName,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mockMLStorage.AddModelIgnoreFile()
		assert.NoError(t, err, "Expected no error when file already exists")
		assert.FileExists(t, modeIgnoreFileName, "Expected the existing model ignore file to remain unchanged")
	})
	t.Run("AddModelIgnoreFile - Failed (Directory creation fails)", func(t *testing.T) {
		originalMkdirAll := mkdirAllFunc
		mkdirAllFunc = func(path string, perm os.FileMode) error {
			return fmt.Errorf("simulated mkdir creation error")
		}
		t.Cleanup(func() {
			mkdirAllFunc = originalMkdirAll
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mockMLStorage := &MLStorage{
			BaseLocalDirectory: filepath.Clean(filepath.FromSlash("invalid/path")),
			mlAlgorithm:        testMlAlgorithmName,
			mlModelConfigName:  testMlModelConfigName,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mockMLStorage.AddModelIgnoreFile()
		assert.Error(t, err, "Expected an error for directory creation failure")
		assert.Contains(t, err.Error(), "simulated mkdir creation error", "Error message should indicate directory creation failure")
	})
}

func TestMLStorage_AddTrainingDataToFile(t *testing.T) {
	mockMLStorage := &MLStorage{
		BaseLocalDirectory: baseTmpFolder,
		mlAlgorithm:        testMlAlgorithmName,
		mlModelConfigName:  testMlModelConfigName,
		lc:                 u.AppService.LoggingClient(),
	}
	headers := []string{"Header1", "Header2", "Header3"}
	data := [][]string{
		{"Row1Col1", "Row1Col2", "Row1Col3"},
		{"Row2Col1", "Row2Col2", "Row2Col3"},
	}
	t.Run("AddTrainingDataToFile - File Does Not Exist", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data"), os.ModePerm)
		testFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data", testMlModelConfigName+".csv")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.AddTrainingDataToFile(data, headers)
		assert.NoError(t, err, "Expected no error when file does not exist")
		assert.FileExists(t, testFileName, "Expected the file to be created")

		fileContent, err := os.ReadFile(testFileName)
		assert.NoError(t, err, "Expected no error reading the file")
		expectedContent := "Header1,Header2,Header3\nRow1Col1,Row1Col2,Row1Col3\nRow2Col1,Row2Col2,Row2Col3\n"
		assert.Equal(t, expectedContent, string(fileContent), "File content does not match expected")
	})
	t.Run("AddTrainingDataToFile - Passed (File exists)", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data"), os.ModePerm)
		testFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data", testMlModelConfigName+".csv")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		// Pre-create file with headers
		err := os.WriteFile(testFileName, []byte("Header1,Header2,Header3\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial file: %v", err)
		}

		err = mockMLStorage.AddTrainingDataToFile(data, headers)
		assert.NoError(t, err, "Expected no error when appending data to an existing file")
		assert.FileExists(t, testFileName, "Expected the file to still exist")

		fileContent, err := os.ReadFile(testFileName)
		assert.NoError(t, err, "Expected no error reading the file")
		expectedContent := "Header1,Header2,Header3\nRow1Col1,Row1Col2,Row1Col3\nRow2Col1,Row2Col2,Row2Col3\n"
		assert.Equal(t, expectedContent, string(fileContent), "File content does not match expected after appending")
	})
}

func TestMLStorage_AddTrainingDataToFileWithFileName(t *testing.T) {
	mockMLStorage := &MLStorage{
		BaseLocalDirectory: baseTmpFolder,
		lc:                 u.AppService.LoggingClient(),
	}
	headers := []string{"Header1", "Header2", "Header3"}
	data := [][]string{
		{"Row1Col1", "Row1Col2", "Row1Col3"},
		{"Row2Col1", "Row2Col2", "Row2Col3"},
	}
	t.Run("AddTrainingDataToFileWithFileName - Passed (File doesn't exist)", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data"), os.ModePerm)
		testFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data", testMlModelConfigName+".csv")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.AddTrainingDataToFileWithFileName(data, headers, testFileName)
		assert.NoError(t, err, "Expected no error when file does not exist")
		assert.FileExists(t, testFileName, "Expected the file to be created")

		fileContent, err := os.ReadFile(testFileName)
		assert.NoError(t, err, "Expected no error reading the file")
		expectedContent := "Header1,Header2,Header3\nRow1Col1,Row1Col2,Row1Col3\nRow2Col1,Row2Col2,Row2Col3\n"
		assert.Equal(t, expectedContent, string(fileContent), "File content does not match expected")
	})
	t.Run("AddTrainingDataToFileWithFileName - Passed (File exists)", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data"), os.ModePerm)
		testFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "data", testMlModelConfigName+".csv")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		// Pre-create file with headers
		err := os.WriteFile(testFileName, []byte("Header1,Header2,Header3\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial file: %v", err)
		}

		err = mockMLStorage.AddTrainingDataToFileWithFileName(data, headers, testFileName)
		assert.NoError(t, err, "Expected no error when appending data to an existing file")
		assert.FileExists(t, testFileName, "Expected the file to still exist")

		fileContent, err := os.ReadFile(testFileName)
		assert.NoError(t, err, "Expected no error reading the file")
		expectedContent := "Header1,Header2,Header3\nRow1Col1,Row1Col2,Row1Col3\nRow2Col1,Row2Col2,Row2Col3\n"
		assert.Equal(t, expectedContent, string(fileContent), "File content does not match expected after appending")
	})
	t.Run("AddTrainingDataToFileWithFileName - Failed (Error while creating file)", func(t *testing.T) {
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		invalidFileName := filepath.Clean(filepath.FromSlash("invalid/path/training_data.csv"))
		err := mockMLStorage.AddTrainingDataToFileWithFileName(data, headers, invalidFileName)
		assert.Error(t, err, "Expected an error when creating file in an invalid path")
		assert.Contains(t, err.Error(), pathNotFoundErrMsg, "Error message should indicate invalid path")
	})
}

func TestMLStorage_CompressFolder(t *testing.T) {
	mockMLStorage := &MLStorage{
		BaseLocalDirectory: baseTmpFolder,
		lc:                 u.AppService.LoggingClient(),
	}
	t.Run("CompressFolder - Passed", func(t *testing.T) {
		testInputFolder := filepath.Join(baseTmpFolder, "input")
		testZipFile := filepath.Join(baseTmpFolder, "output.zip")
		_ = os.MkdirAll(testInputFolder, os.ModePerm)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		testFile1 := filepath.Join(testInputFolder, "file1.txt")
		testFile2 := filepath.Join(testInputFolder, "file2.txt")
		_ = os.WriteFile(testFile1, []byte("Content of file 1"), 0644)
		_ = os.WriteFile(testFile2, []byte("Content of file 2"), 0644)

		err := mockMLStorage.CompressFolder(testInputFolder, testZipFile)
		assert.NoError(t, err, "Expected no error during folder compression")
		assert.FileExists(t, testZipFile, "Expected the zip file to be created")

		// Verify contents of the zip file
		zipReader, err := zip.OpenReader(testZipFile)
		assert.NoError(t, err, "Expected no error opening the zip file")
		defer zipReader.Close()

		var fileNames []string
		for _, file := range zipReader.File {
			fileNames = append(fileNames, file.Name)
		}

		expectedFileNames := []string{filepath.Join("input", "file1.txt"), filepath.Join("input", "file2.txt")}
		assert.ElementsMatch(t, expectedFileNames, fileNames, "File names in zip do not match expected")
	})
	t.Run("CompressFolder - Failed (Empty folder)", func(t *testing.T) {
		testInputFolder := filepath.Join(baseTmpFolder, "empty")
		testZipFile := filepath.Join(baseTmpFolder, "output_empty.zip")
		_ = os.MkdirAll(testInputFolder, os.ModePerm)
		_ = mockMLStorage.CompressFolder(testInputFolder, testZipFile)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		// Verify the zip file exists and is empty
		assert.FileExists(t, testZipFile, "Expected the zip file to be created")
		zipReader, err := zip.OpenReader(testZipFile)
		assert.NoError(t, err, "Expected no error opening the zip file")
		defer zipReader.Close()

		assert.Empty(t, zipReader.File, "Expected no files in the zip archive")
	})
	t.Run("CompressFolder - Failed (Invalid folder path)", func(t *testing.T) {
		testInputFolder := filepath.Join(baseTmpFolder, "non_existent")
		testZipFile := filepath.Join(baseTmpFolder, "output_invalid.zip")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.CompressFolder(testInputFolder, testZipFile)
		assert.Error(t, err, "Expected error due to invalid input folder path")
		assert.Contains(t, err.Error(), fmt.Sprintf("open %s: %s", testZipFile, pathNotFoundErrMsg), "Unexpected error message")
	})
	t.Run("CompressFolder - Failed (File open error)", func(t *testing.T) {
		originalOpenFile := openFile
		openFile = func(path string) (*os.File, error) {
			return nil, fmt.Errorf("simulated file opening error")
		}
		testInputFolder := filepath.Join(baseTmpFolder, "input_with_error")
		testZipFile := filepath.Join(baseTmpFolder, "output_with_error.zip")
		_ = os.MkdirAll(testInputFolder, os.ModePerm)
		testFile := filepath.Join(testInputFolder, "file1.txt")
		_ = os.WriteFile(testFile, []byte("Content of file 1"), 0644)
		t.Cleanup(func() {
			openFile = originalOpenFile
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.CompressFolder(testInputFolder, testZipFile)
		assert.Error(t, err, "Expected error due to file open error")
		assert.Contains(t, err.Error(), "simulated file opening error", "Unexpected error message")
	})
}

func TestMLStorage_CompressFiles(t *testing.T) {
	mlStorage := &MLStorage{
		lc: u.AppService.LoggingClient(),
	}
	t.Run("CompressFiles - Passed", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		inputFiles := []ZipFileInfo{
			{
				FilePath:      filepath.Join(baseTmpFolder, "file1.txt"),
				FilePathInZip: "file1.txt",
			},
			{
				FilePath:      filepath.Join(baseTmpFolder, "file2.txt"),
				FilePathInZip: filepath.Join("nested", "file2.txt"),
			},
		}
		for _, file := range inputFiles {
			_ = os.MkdirAll(filepath.Dir(file.FilePath), os.ModePerm)
			_ = os.WriteFile(file.FilePath, []byte("dummy content"), 0644)
		}

		zipFilePath := filepath.Join(baseTmpFolder, "output.zip")

		err := mlStorage.CompressFiles(inputFiles, zipFilePath)
		assert.NoError(t, err, "Expected no error during compression")
		assert.FileExists(t, zipFilePath, "Expected zip file to be created")

		// Verify zip file contents
		zipReader, err := zip.OpenReader(zipFilePath)
		assert.NoError(t, err, "Expected no error opening zip file")
		defer zipReader.Close()

		fileNames := make(map[string]bool)
		for _, f := range zipReader.File {
			fileNames[f.Name] = true
		}
		for _, file := range inputFiles {
			assert.Contains(t, fileNames, file.FilePathInZip, "Expected file in zip: %s", file.FilePathInZip)
		}
	})
	t.Run("CompressFiles - Failed (Missing input file)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		// Define input files with one missing file
		inputFiles := []ZipFileInfo{
			{
				FilePath:      filepath.Join(baseTmpFolder, "file1.txt"),
				FilePathInZip: "file1.txt",
			},
			{
				FilePath:      filepath.Join(baseTmpFolder, "missing_file.txt"),
				FilePathInZip: "missing_file.txt",
			},
		}
		_ = os.WriteFile(inputFiles[0].FilePath, []byte("dummy content"), 0644)

		zipFilePath := filepath.Join(baseTmpFolder, "output.zip")

		err := mlStorage.CompressFiles(inputFiles, zipFilePath)
		assert.Error(t, err, "Expected error due to missing input file")
	})
	t.Run("CompressFiles - Failed (Invalid zip file path)", func(t *testing.T) {
		zipFilePath := string([]byte{0x00, 0x01, 0x02}) // Invalid file name

		err := mlStorage.CompressFiles([]ZipFileInfo{}, zipFilePath)
		assert.Error(t, err, "Expected error due to missing input file")
	})
}

func TestMLStorage_GetValidationLocalDir(t *testing.T) {
	mockMLStorage := &MLStorage{
		BaseLocalDirectory: baseTmpFolder,
		mlAlgorithm:        testMlAlgorithmName,
		mlModelConfigName:  testMlModelConfigName,
	}
	expectedPath := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "validation")
	t.Run("GetValidationLocalDir - Passed", func(t *testing.T) {
		actualPath := mockMLStorage.GetValidationLocalDir()
		assert.Equal(t, expectedPath, actualPath, "Validation local directory path does not match expected")
	})
}

func TestMLStorage_GeneratorRemoteTrainingDataZipFileName(t *testing.T) {
	storage := &MLStorage{
		mlAlgorithm:       testMlAlgorithmName,
		mlModelConfigName: testMlModelConfigName,
	}

	generatedFileName := storage.GeneratorRemoteTrainingDataZipFileName()

	expectedPrefix := filepath.Join(testMlAlgorithmName, testMlModelConfigName, "data_")
	assert.True(t, strings.HasPrefix(generatedFileName, expectedPrefix), "File name does not start with the expected prefix")
	// verify timestamp's format
	timestampPart := strings.TrimSuffix(strings.TrimPrefix(generatedFileName, expectedPrefix), ".zip")
	_, err := strconv.ParseInt(timestampPart, 10, 64)
	assert.NoError(t, err, "Timestamp part is not a valid integer")
	assert.True(t, strings.HasSuffix(generatedFileName, ".zip"), "File name does not end with the expected '.zip'")
}

func TestMLStorage_GetBaseLocalDirectory(t *testing.T) {
	baseLocalDirectory := filepath.Clean(filepath.FromSlash("path/to/base/directory"))
	s := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
	}
	result := s.GetBaseLocalDirectory()
	if result != baseLocalDirectory {
		t.Errorf("Expected base local directory: %s, but got: %s", baseLocalDirectory, result)
	}
}

func TestMLStorage_GetModelLocalZipFileName(t *testing.T) {
	baseLocalDirectory := filepath.Clean(filepath.FromSlash("path/to/base/directory"))
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
		mlAlgorithm:        mlAlgorithm,
		mlModelConfigName:  mlTrainingDataConfig,
	}
	expectedFileName := filepath.Join(baseLocalDirectory, mlAlgorithm, mlTrainingDataConfig, MODEL_ZIP_FILENAME)
	actualFileName := s.GetModelLocalZipFileName()
	if actualFileName != expectedFileName {
		t.Errorf("Expected model file name: %s, but got: %s", expectedFileName, actualFileName)
	}
}

func TestMLStorage_GetModelZipFile(t *testing.T) {
	baseLocalDirectory := filepath.Clean(filepath.FromSlash("path/to/base/directory"))
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
		mlAlgorithm:        mlAlgorithm,
		mlModelConfigName:  mlTrainingDataConfig,
	}

	expectedFileName := filepath.Join(baseLocalDirectory, mlAlgorithm, mlTrainingDataConfig, MODEL_ZIP_FILENAME)
	actualFileName := s.GetModelZipFile()
	if actualFileName != expectedFileName {
		t.Errorf("Expected model zip file name: %s, but got: %s", expectedFileName, actualFileName)
	}
}

func TestMLStorage_GetRemoteConfigFileName(t *testing.T) {
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		mlAlgorithm:       mlAlgorithm,
		mlModelConfigName: mlTrainingDataConfig,
	}
	expectedFileName := filepath.Join(mlAlgorithm, mlTrainingDataConfig, "assets", "config.json")
	actualFileName := s.GetRemoteConfigFileName()
	if actualFileName != expectedFileName {
		t.Errorf("Expected remote config file name: %s, but got: %s", expectedFileName, actualFileName)
	}
}

func TestMLStorage_GetRemoteTrainingDataFileName(t *testing.T) {
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		mlAlgorithm:       mlAlgorithm,
		mlModelConfigName: mlTrainingDataConfig,
	}

	expectedFileName := filepath.Join(mlAlgorithm, mlTrainingDataConfig, "training_data", "training_data.csv")
	actualFileName := s.GetRemoteTrainingDataFileName()
	if actualFileName != expectedFileName {
		t.Errorf("Expected remote training data file name: %s, but got: %s", expectedFileName, actualFileName)
	}
}

func TestMLStorage_GetTrainingConfigFileName(t *testing.T) {
	baseLocalDirectory := filepath.Clean(filepath.FromSlash("path/to/base/directory"))
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
		mlAlgorithm:        mlAlgorithm,
		mlModelConfigName:  mlTrainingDataConfig,
	}

	expectedFileNameForDataGeneration := filepath.Join(s.GetLocalTrainingDataBaseDir(), "config.json")
	actualFileNameForDataGeneration := s.GetMLModelConfigFileName(true)

	if actualFileNameForDataGeneration != expectedFileNameForDataGeneration {
		t.Errorf("Expected training config file name for data generation: %s, but got: %s",
			expectedFileNameForDataGeneration, actualFileNameForDataGeneration)
	}

	expectedFileNameForModelTraining := filepath.Join(s.GetModelLocalDir(), "config.json")
	actualFileNameForModelTraining := s.GetMLModelConfigFileName(false)

	if actualFileNameForModelTraining != expectedFileNameForModelTraining {
		t.Errorf("Expected training config file name for model training: %s, but got: %s",
			expectedFileNameForModelTraining, actualFileNameForModelTraining)
	}
}

func TestMLStorage_GetTrainingInputZipFile(t *testing.T) {
	baseLocalDirectory := filepath.Clean(filepath.FromSlash("path/to/base/directory"))
	mlAlgorithm := "my_algorithm"
	mlTrainingDataConfig := "config123"

	s := &MLStorage{
		BaseLocalDirectory: baseLocalDirectory,
		mlAlgorithm:        mlAlgorithm,
		mlModelConfigName:  mlTrainingDataConfig,
	}

	expectedFileName := filepath.Join(baseLocalDirectory, mlAlgorithm, mlTrainingDataConfig, "training_input.zip")
	actualFileName := s.GetTrainingInputZipFile()
	if actualFileName != expectedFileName {
		t.Errorf("Expected training input zip file name: %s, but got: %s", expectedFileName, actualFileName)
	}
}

func TestMLStorage_InitializeFile(t *testing.T) {
	testFileName := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "testfile.txt")
	mockMLStorage := &MLStorage{
		BaseLocalDirectory: baseTmpFolder,
		mlAlgorithm:        testMlAlgorithmName,
		mlModelConfigName:  testMlModelConfigName,
		lc:                 u.AppService.LoggingClient(),
	}

	t.Run("InitializeFile - Passed (File and directory don't exist)", func(t *testing.T) {
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.InitializeFile(testFileName)
		assert.NoError(t, err, "Expected no error when initializing file")
		assert.DirExists(t, filepath.Dir(testFileName), "Expected directory to be created")
		assert.False(t, mockMLStorage.FileExists(testFileName), "File should not exist after initialization")
	})
	t.Run("InitializeFile - Passed (File exists)", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Dir(testFileName), os.ModePerm)
		_ = os.WriteFile(testFileName, []byte("dummy content"), 0644)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := mockMLStorage.InitializeFile(testFileName)
		assert.NoError(t, err, "Expected no error when initializing file")
		assert.False(t, mockMLStorage.FileExists(testFileName), "File should be removed during initialization")
	})
}

func TestMLStorage_RemoveModelIgnoreFile(t *testing.T) {
	t.Run("RemoveModelIgnoreFile - Passed (File exists and removed)", func(t *testing.T) {
		testModelIgnoreFilePath := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "hedge_export", ".modelignore")
		_ = os.MkdirAll(filepath.Dir(testModelIgnoreFilePath), os.ModePerm)
		_ = os.WriteFile(testModelIgnoreFilePath, []byte("dummy content"), 0644)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mockMLStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			mlAlgorithm:        testMlAlgorithmName,
			mlModelConfigName:  testMlModelConfigName,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mockMLStorage.RemoveModelIgnoreFile()
		assert.NoError(t, err, "Expected no error when file is successfully removed")
		assert.False(t, mockMLStorage.FileExists(testModelIgnoreFilePath), "Expected the file to be removed")
	})
	t.Run("RemoveModelIgnoreFile - Passed (File does not exist)", func(t *testing.T) {
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		mockMLStorage := &MLStorage{
			BaseLocalDirectory: baseTmpFolder,
			mlAlgorithm:        testMlAlgorithmName,
			mlModelConfigName:  testMlModelConfigName,
			lc:                 u.AppService.LoggingClient(),
		}

		err := mockMLStorage.RemoveModelIgnoreFile()
		assert.NoError(t, err, "Expected no error when file does not exist")
	})
}

func TestMLStorage_ReadMLConfigWithAlgo(t *testing.T) {
	t.Run("ReadMLConfigWithAlgo - Failed (Invalid base path)", func(t *testing.T) {
		basePath := filepath.Join("invalid", "path")

		result, err := ReadMLConfigWithAlgo(basePath, u.AppService.LoggingClient(), "")
		assert.Error(t, err, "Expected error for invalid base path")
		assert.Nil(t, result, "Expected result to be nil for invalid base path")
	})
	t.Run("ReadMLConfigWithAlgo - Valid Configurations", func(t *testing.T) {
		testHedgeExportDir := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName, "hedge_export", "assets")
		_ = os.MkdirAll(testHedgeExportDir, os.ModePerm)
		_ = os.WriteFile(filepath.Join(testHedgeExportDir, "config.json"), []byte(testConfigContent), 0644)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		result, err := ReadMLConfigWithAlgo(baseTmpFolder, u.AppService.LoggingClient(), "")
		assert.NoError(t, err, "Expected no error for valid configurations")
		assert.NotNil(t, result, "Expected result to be non-nil for valid configurations")
		assert.Contains(t, result, testMlModelConfigName, "Expected config for 'testModel'")
		assert.Equal(t, filepath.Join(baseTmpFolder, testMlModelConfigName, "hedge_export"), result[testMlModelConfigName].MLModelConfig.LocalModelStorageDir, "Local storage dir mismatch")
	})
	t.Run("ReadMLConfigWithAlgo - Passed (Algo name filter)", func(t *testing.T) {
		testHedgeExportDir1 := filepath.Join(baseTmpFolder, testMlAlgorithmName+"1", testMlModelConfigName, "hedge_export/assets")
		_ = os.MkdirAll(testHedgeExportDir1, os.ModePerm)
		testHedgeExportDir2 := filepath.Join(baseTmpFolder, testMlAlgorithmName+"2", testMlModelConfigName, "hedge_export/assets")
		_ = os.MkdirAll(testHedgeExportDir2, os.ModePerm)
		_ = os.WriteFile(filepath.Join(testHedgeExportDir1, "config.json"), []byte(testConfigContent), 0644)
		_ = os.WriteFile(filepath.Join(testHedgeExportDir2, "config.json"), []byte(testConfigContent), 0644)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		result, err := ReadMLConfigWithAlgo(baseTmpFolder, u.AppService.LoggingClient(), testMlAlgorithmName+"1")
		assert.NoError(t, err, "Expected no error for valid configurations with filter")
		assert.NotNil(t, result, "Expected result to be non-nil for valid configurations with filter")
		assert.Contains(t, result, testMlModelConfigName, "Expected config for 'TestMLModelConfig' in filtered algorithm")
	})
	t.Run("ReadMLConfigWithAlgo - Passed (Ignore models with .modelignore)", func(t *testing.T) {
		testModelDir := filepath.Join(baseTmpFolder, testMlAlgorithmName, testMlModelConfigName)
		testHedgeExportDir := filepath.Join(testModelDir, "hedge_export/assets")
		_ = os.MkdirAll(testHedgeExportDir, os.ModePerm)
		_ = os.WriteFile(filepath.Join(testModelDir, "hedge_export/.modelignore"), []byte{}, 0644)
		_ = os.WriteFile(filepath.Join(testHedgeExportDir, "config.json"), []byte(testConfigContent), 0644)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		result, err := ReadMLConfigWithAlgo(baseTmpFolder, u.AppService.LoggingClient(), "")
		assert.NoError(t, err, "Expected no error for valid configurations with .modelignore")
		assert.NotContains(t, result, "testModel", "Expected 'testModel' to be ignored due to .modelignore")
	})
}

func TestMLStorage_ReadZipFiles(t *testing.T) {
	t.Run("ReadZipFiles - Passed", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testZipFilePath := filepath.Join(baseTmpFolder, "test.zip")
		testFiles := map[string]string{
			"file1.txt": "Hello, this is file 1",
			"file2.txt": "Hello, this is file 2",
		}
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		buf := new(bytes.Buffer)
		zipWriter := zip.NewWriter(buf)

		for fileName, content := range testFiles {
			f, err := zipWriter.Create(fileName)
			if err != nil {
				t.Fatalf("Failed to create file in zip: %v", err)
			}
			_, err = f.Write([]byte(content))
			if err != nil {
				t.Fatalf("Failed to write content to zip: %v", err)
			}
		}
		err := zipWriter.Close()
		if err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}
		err = os.WriteFile(testZipFilePath, buf.Bytes(), 0644)
		if err != nil {
			t.Fatalf("Failed to write zip file: %v", err)
		}

		zipFiles, err := ReadZipFiles(testZipFilePath)
		assert.NoError(t, err, "Expected no error when reading valid zip file")
		assert.Len(t, zipFiles, len(testFiles), "Number of files in zip does not match expected")
		for _, file := range zipFiles {
			assert.Contains(t, testFiles, file.Path, "Unexpected file in zip")
		}
	})
	t.Run("ReadZipFiles - Failed (Invalid filePath)", func(t *testing.T) {
		invalidFilePath := "nonexistent.zip"
		_, err := ReadZipFiles(invalidFilePath)
		assert.Error(t, err, "Expected an error for nonexistent zip file")
		assert.Contains(t, err.Error(), fileNotFoundErrMsg, "Error message does not indicate missing file")
	})
	t.Run("ReadZipFiles - Failed (Corrupted zip file)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testZipFilePath := filepath.Join(baseTmpFolder, "corrupted.zip")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})
		_ = os.WriteFile(testZipFilePath, []byte("this is not a zip file"), 0644)

		_, err := ReadZipFiles(testZipFilePath)
		assert.Error(t, err, "Expected an error for corrupted zip file")
		assert.Contains(t, err.Error(), "not a valid zip file", "Error message does not indicate corrupted zip")
	})
}

func TestMLStorage_SaveFile(t *testing.T) {
	t.Run("SaveFile - Passed", func(t *testing.T) {
		filePath := filepath.Join(baseTmpFolder, "test_dir", "test_file.txt")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		data := []byte("This is test content.")

		err := SaveFile(filePath, data)
		assert.NoError(t, err, "Expected no error when saving file")
		assert.FileExists(t, filePath, "Expected file to exist after saving")
		fileContent, err := os.ReadFile(filePath)
		assert.NoError(t, err, "Expected no error reading the saved file")
		assert.Equal(t, data, fileContent, "File content does not match expected data")
	})
	t.Run("SaveFile - Failed (Directory creation failure)", func(t *testing.T) {
		originalMkdirAll := mkdirAllFunc
		mkdirAllFunc = func(path string, perm os.FileMode) error {
			return fmt.Errorf("simulated mkdir creation error")
		}
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testFilePath := filepath.Join(baseTmpFolder, "test_dir")
		data := []byte("This is test content.")
		nestedFilePath := filepath.Join(testFilePath, "test_file.txt")
		t.Cleanup(func() {
			mkdirAllFunc = originalMkdirAll
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := SaveFile(nestedFilePath, data)
		assert.Error(t, err, "Expected an error when saving file in a nested directory")
		assert.Contains(t, err.Error(), "simulated mkdir creation error", "Error message does not indicate file creation failure")
	})
	t.Run("SaveFile - Failed (File creation failure)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testFilePath := filepath.Join(baseTmpFolder, "test_dir")
		err := os.WriteFile(testFilePath, []byte("dummy content"), os.ModePerm) // Create a file instead of a directory
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		data := []byte("This is test content.")
		nestedFilePath := filepath.Join(testFilePath, "test_file.txt")

		err = SaveFile(nestedFilePath, data)
		assert.Error(t, err, "Expected an error when saving file in a nested directory")
		assert.Contains(t, err.Error(), notADirectory, "Error message does not indicate file creation failure")
	})
}

func TestMLStorage_SaveMultipartFile(t *testing.T) {
	t.Run("SaveMultipartFile - Passed", func(t *testing.T) {
		testFileName := filepath.Join(baseTmpFolder, "uploaded", "file.txt")
		_ = os.MkdirAll(filepath.Dir(testFileName), os.ModePerm)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		testContent := []byte("This is test content.")
		testSrcFile := createMultipartFile(t, testContent)
		testSrcFile.Close()

		err := SaveMultipartFile(testSrcFile, testFileName)
		assert.NoError(t, err, "Expected no error when saving multipart file")

		savedContent, err := os.ReadFile(testFileName)
		assert.NoError(t, err, "Expected no error when reading saved file")
		assert.Equal(t, testContent, savedContent, "Saved file content does not match expected content")
	})
	t.Run("SaveMultipartFile - Failed (Directory creation failure)", func(t *testing.T) {
		originalMkdirAll := mkdirAllFunc
		mkdirAllFunc = func(path string, perm os.FileMode) error {
			return fmt.Errorf("simulated mkdir creation error")
		}
		testFileName := filepath.Join(baseTmpFolder, "test-dir", "file.txt")
		_ = os.Mkdir(filepath.Join(baseTmpFolder, "test-dir"), os.ModePerm)
		t.Cleanup(func() {
			mkdirAllFunc = originalMkdirAll
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		testContent := []byte("This is test content.")
		testSrcFile := createMultipartFile(t, testContent)
		testSrcFile.Close()

		err := SaveMultipartFile(testSrcFile, testFileName)
		assert.Error(t, err, "Expected error when saving multipart file in a read-only directory")
		assert.Contains(t, err.Error(), "simulated mkdir creation error", "Error message does not indicate directory creation failure")
	})
	t.Run("SaveMultipartFile - Failed (File creation failure)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testFileName := filepath.Join(baseTmpFolder, "invalid", "file.txt")
		_ = os.WriteFile(filepath.Join(baseTmpFolder, "invalid"), []byte("dummy content"), os.ModePerm)
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		testContent := []byte("This is test content.")
		testSrcFile := createMultipartFile(t, testContent)
		testSrcFile.Close()

		err := SaveMultipartFile(testSrcFile, testFileName)
		assert.Error(t, err, "Expected error when saving multipart file with invalid file path")
		assert.Contains(t, err.Error(), notADirectory, "Error message does not indicate file creation failure")
	})
}

func TestMLStorage_ExtractZipFile(t *testing.T) {
	t.Run("ExtractZipFile - Passed", func(t *testing.T) {
		testZipFilePath := filepath.Join(baseTmpFolder, "test.zip")
		testInZipFile := "file.txt"
		testDstPath := filepath.Join(baseTmpFolder, "extracted.txt")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		createDummyZip(t, baseTmpFolder, map[string]bool{
			testInZipFile: true,
		})

		err := ExtractZipFile(testZipFilePath, testInZipFile, testDstPath)
		assert.NoError(t, err, "Expected no error when extracting the file")
	})
	t.Run("ExtractZipFile - Failed (File not found in zip)", func(t *testing.T) {
		testZipFilePath := filepath.Join(baseTmpFolder, "test.zip")
		testInZipFile := "nonexistent.txt"
		testDstPath := filepath.Join(baseTmpFolder, "extracted.txt")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		createDummyZip(t, baseTmpFolder, map[string]bool{
			testInZipFile: false,
		})

		err := ExtractZipFile(testZipFilePath, testInZipFile, testDstPath)
		assert.Error(t, err, "Expected error when file is not found in the zip")
		assert.Contains(t, err.Error(), fmt.Sprintf("file %s not found in zip %s", testInZipFile, testZipFilePath), "Error message does not match expected")
	})
	t.Run("ExtractZipFile - Failed (Invalid zip file)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testZipFilePath := filepath.Join(baseTmpFolder, "invalid.zip")
		testInZipFile := "file.txt"
		testDstPath := filepath.Join(baseTmpFolder, "extracted.txt")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		_ = os.WriteFile(testZipFilePath, []byte("Not a zip file"), 0644)

		err := ExtractZipFile(testZipFilePath, testInZipFile, testDstPath)
		assert.Error(t, err, "Expected error when processing an invalid zip file")
		assert.Contains(t, err.Error(), "not a valid zip file", "Error message does not indicate invalid zip file")
	})
}

func TestMLStorage_ReadFileData(t *testing.T) {
	t.Run("ReadFileData - Passed", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testFilePath := filepath.Join(baseTmpFolder, "test.txt")
		expectedContent := "This is a test file content."
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		_ = os.WriteFile(testFilePath, []byte(expectedContent), 0644)

		data, err := ReadFileData(testFilePath)
		assert.NoError(t, err, "Expected no error when reading an existing file")
		assert.Equal(t, expectedContent, string(data), "File content does not match expected")
	})
	t.Run("ReadFileData - Failed (File does not exist)", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		nonExistentFilePath := filepath.Join(baseTmpFolder, "nonexistent.txt")
		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		data, err := ReadFileData(nonExistentFilePath)
		assert.Error(t, err, "Expected an error when reading a non-existent file")
		assert.Nil(t, data, "Expected no data when file does not exist")
		assert.Contains(t, err.Error(), fileNotFoundErrMsg, "Error message does not indicate file does not exist")
	})
}

func TestMLStorage_CopyFile(t *testing.T) {
	t.Run("CopyFile - Passed", func(t *testing.T) {
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testSrcFilePath := filepath.Join(baseTmpFolder, "source.txt")
		testDstFilePath := filepath.Join(baseTmpFolder, "destination", "copied.txt")
		expectedContent := "This is the content of the source file."
		_ = os.WriteFile(testSrcFilePath, []byte(expectedContent), 0644)

		t.Cleanup(func() {
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := CopyFile(testSrcFilePath, testDstFilePath)
		assert.NoError(t, err, "Expected no error during file copy")

		assert.FileExists(t, testDstFilePath, "Expected the destination file to exist")
		copiedContent, err := os.ReadFile(testDstFilePath)
		assert.NoError(t, err, "Expected no error when reading the destination file")
		assert.Equal(t, expectedContent, string(copiedContent), "Destination file content does not match source")
	})
	t.Run("CopyFile - Failed (Source file doesn't exist)", func(t *testing.T) {
		nonExistentSrcPath := filepath.Join(baseTmpFolder, "nonexistent.txt")
		testDstFilePath := filepath.Join(baseTmpFolder, "destination", "copied.txt")

		err := CopyFile(nonExistentSrcPath, testDstFilePath)
		assert.Error(t, err, "Expected an error when source file does not exist")
	})
	t.Run("CopyFile - Failed (Destination directory creation failure)", func(t *testing.T) {
		originalMkdirAll := mkdirAllFunc
		mkdirAllFunc = func(path string, perm os.FileMode) error {
			return fmt.Errorf("simulated mkdir creation error")
		}
		_ = os.MkdirAll(baseTmpFolder, os.ModePerm)
		testSrcFilePath := filepath.Join(baseTmpFolder, "source.txt")
		testDstFilePath := filepath.Join(baseTmpFolder, "readonly", "copied.txt")
		expectedContent := "This is the content of the source file."
		_ = os.WriteFile(testSrcFilePath, []byte(expectedContent), 0644)

		t.Cleanup(func() {
			mkdirAllFunc = originalMkdirAll
			err := os.RemoveAll(baseTmpFolder)
			if err != nil {
				t.Errorf("Failed to cleanup test directory: %v", err)
			}
		})

		err := CopyFile(testSrcFilePath, testDstFilePath)
		assert.Error(t, err, "Expected an error due to directory creation failure")
	})
}
