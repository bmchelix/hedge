package docker

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"os"
	"path/filepath"
	"testing"
)

var (
	u                 *utils.HedgeMockUtils
	testMlModelDir    = "testMlModelDir"
	testRegistry      = "registry.test.com"
	testRegistryUser  = "TestUser"
	testRegistryPass  = "TestPassword"
	testAlgorithmName = "TestAlgo"
	testImagePath     = "Test-image"
	testContainerID   = "Test-container-id"
	testContainerName = "Test-container"
	testVolumeName    = "Test-volume"
	testError         error
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{})
	testError = errors.New("dummy error")
}

// objectNotFoundError simulates the docker client's objectNotFoundError error
type objectNotFoundError struct {
	object string
	id     string
}

func (e objectNotFoundError) NotFound() {}

func (e objectNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such %s: %s", e.object, e.id)
}

// createTestFiles creates a test directory with test files
func createTestFiles(t *testing.T, testDir string) {
	err := os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	// Create files for the algorithm
	file1 := filepath.Join(testDir, testAlgorithmName+"_file1.txt")
	err = os.WriteFile(file1, []byte("content1"), 0644)
	assert.NoError(t, err)

	file2 := filepath.Join(testDir, testAlgorithmName+"_file2.txt")
	err = os.WriteFile(file2, []byte("content2"), 0644)
	assert.NoError(t, err)

	// Create unrelated files
	unrelated := filepath.Join(testDir, "unrelated_file.txt")
	err = os.WriteFile(unrelated, []byte("unrelated"), 0644)
	assert.NoError(t, err)
}
