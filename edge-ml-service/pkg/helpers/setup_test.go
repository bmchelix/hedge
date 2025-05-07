package helpers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/stretchr/testify/assert"
)

const baseTmpFolder = "tmp"

var (
	u *utils.HedgeMockUtils
	// testError             = errors.New("dummy error")
	testMlAlgorithmName   = "TestAlgo"
	testMlModelConfigName = "TestMLModelConfig"
	testMlModelDir        = "testMlModelDir"
	testImagePath         = "test-image:test-tag"
	testImageDigest       = "sha256:dummydigest"
	testGetDigestURL      = "https://test-image-repo/v2/iot/test-image-name/manifests/test-tag"
	testRegistryCreds     = RegistryCredentials{}
	testConfigContent     = `{
								"mlModelConfig": {
									"name": "TestMLModelConfig",
									"localModelStorageDir": ""
								}
							}`
	fileNotFoundErrMsg = "no such file or directory"
	pathNotFoundErrMsg = "no such file or directory"
	notADirectory      = "not a directory"
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{
		"RedisHost":     "localhost",
		"RedisPort":     "6379",
		"RedisName":     "metadata",
		"RedisTimeout":  "5000",
		"ConsulURL":     "localhost",
		"ImageRegistry": "test-image-repo/iot/",
	})
	testRegistryCreds = RegistryCredentials{
		RegistryURL: "test-image-repo/iot/",
		UserName:    "username",
		Password:    "password",
	}

	if runtime.GOOS == "windows" {
		fileNotFoundErrMsg = "The system cannot find the file specified"
		pathNotFoundErrMsg = "The system cannot find the path specified"
		notADirectory = "The system cannot find the path specified"
	}
}

func buildTestMlAlgoDefinition() config.MLAlgorithmDefinition {
	return config.MLAlgorithmDefinition{
		Name:                     testMlAlgorithmName,
		Description:              "Test description",
		Type:                     ANOMALY_ALGO_TYPE,
		Enabled:                  true,
		OutputFeaturesPredefined: true,
	}
}

func buildTestMLModelConfig() *config.MLModelConfig {
	return &config.MLModelConfig{
		Name:                 testMlModelConfigName,
		Version:              "1",
		Description:          "Test description",
		MLAlgorithm:          testMlAlgorithmName,
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: testMlModelDir,
		TrainedModelCount:    0,
	}
}

func jsonDeepEqual(a, b []byte) bool {
	var j1, j2 interface{}
	if err := json.Unmarshal(a, &j1); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &j2); err != nil {
		return false
	}
	return reflect.DeepEqual(j1, j2)
}

func createDummyZip(t *testing.T, testDir string, files map[string]bool) *bytes.Buffer {
	t.Helper()

	err := os.MkdirAll(testDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for fileName, isValid := range files {
		var err error
		if isValid {
			_, err = zipWriter.Create(fileName)
		} else {
			_, err = zipWriter.Create("../" + fileName) // Add an invalid file path (e.g., path traversal or inaccessible directory)
		}

		if err != nil {
			t.Fatalf("Failed to create zip entry for %s: %v", fileName, err)
		}
	}

	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	zipFilePath := filepath.Join(testDir, "test.zip")
	err = os.WriteFile(zipFilePath, buf.Bytes(), 0644)
	if err != nil {
		t.Fatalf("Failed to write zip file: %v", err)
	}

	return buf
}

func createMultipartFile(t *testing.T, content []byte) multipart.File {
	t.Helper()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	assert.NoError(t, err, "Expected no error while creating multipart file")
	_, err = part.Write(content)
	assert.NoError(t, err, "Expected no error while writing to multipart file")
	writer.Close()

	srcFile, err := multipart.NewReader(buf, writer.Boundary()).ReadForm(100)
	assert.NoError(t, err, "Expected no error while reading multipart form")

	file, err := srcFile.File["file"][0].Open()
	return file
}
