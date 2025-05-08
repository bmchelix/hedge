package docker

import (
	"archive/tar"
	dockermocks "hedge/mocks/hedge/edge-ml-service/pkg/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"os"
	"strings"
	"testing"
)

func TestContainerManagerDocker_NewDockerContainerManager(t *testing.T) {
	t.Run("NewDockerContainerManager - Passed", func(t *testing.T) {
		cm, err := NewDockerContainerManager(u.AppService, testMlModelDir, testRegistry, testRegistryUser, testRegistryPass, true)
		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})
}

func TestContainerManagerDocker_CreatePredicationContainer(t *testing.T) {
	t.Run("CreatePredicationContainer - Passed", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, nil, nil)
		mockedDockerClient.On("VolumeCreate", mock.Anything, mock.Anything).Return(volume.Volume{Name: testVolumeName}, nil)
		mockedDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, testContainerName).
			Return(container.CreateResponse{ID: testContainerID}, nil)

		containerManagerDocker := &ContainerManagerDocker{
			client:              mockedDockerClient,
			exposeContainerPort: true,
		}

		containerID, err := containerManagerDocker.CreatePredicationContainer(testImagePath, testAlgorithmName, 0, testContainerName)

		assert.NoError(t, err)
		assert.Equal(t, testContainerID, containerID)
	})
	t.Run("CreatePredicationContainer - Failed (Failed to Inspect Image)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, nil, objectNotFoundError{})
		mockedDockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		_, err := containerManagerDocker.CreatePredicationContainer(testImagePath, testAlgorithmName, 0, testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image: dummy error")
	})
	t.Run("CreatePredicationContainer - Failed (Failed to Inspect Image + failed to create volume)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, nil, objectNotFoundError{})
		mockedDockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(io.NopCloser(strings.NewReader("")), nil)
		mockedDockerClient.On("VolumeCreate", mock.Anything, mock.Anything).Return(volume.Volume{}, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		_, err := containerManagerDocker.CreatePredicationContainer(testImagePath, testAlgorithmName, 0, testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create volume: dummy error")
	})
	t.Run("CreatePredicationContainer - Failed (Failed to create volume)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, nil, nil)
		mockedDockerClient.On("VolumeCreate", mock.Anything, mock.Anything).Return(volume.Volume{}, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		_, err := containerManagerDocker.CreatePredicationContainer(testImagePath, testAlgorithmName, 0, testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create volume: dummy error")
	})
	t.Run("CreatePredicationContainer - Failed (Failed to create container)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, nil, nil)
		mockedDockerClient.On("VolumeCreate", mock.Anything, mock.Anything).Return(volume.Volume{Name: testVolumeName}, nil)
		mockedDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, testContainerName).
			Return(container.CreateResponse{}, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		_, err := containerManagerDocker.CreatePredicationContainer(testImagePath, testAlgorithmName, 0, testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create image: dummy error")
	})
}

func TestContainerManagerDocker_CopyModelToContainer(t *testing.T) {
	t.Run("CopyModelToContainer - Passed", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("CopyToContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}
		err := containerManagerDocker.CopyModelToContainer(testContainerID, testAlgorithmName)

		assert.NoError(t, err)
	})
	t.Run("CopyModelToContainer - Failed", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("CopyToContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}
		err := containerManagerDocker.CopyModelToContainer(testContainerID, testAlgorithmName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy to container")
	})
}

func TestContainerManagerDocker_RemoveContainer(t *testing.T) {
	t.Run("RemoveContainer - Passed", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
			{ID: testContainerID, Names: []string{"/" + testContainerName}},
		}, nil)
		mockedDockerClient.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("VolumeRemove", mock.Anything, mock.Anything, true).Return(nil)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.RemoveContainer(testContainerName)

		assert.NoError(t, err)
	})
	t.Run("RemoveContainer - Failed (failed to list containers)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(nil, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.RemoveContainer(testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list containers")
	})
	t.Run("RemoveContainer - Failed (failed to remove volume)", func(t *testing.T) {
		testContainers := []types.Container{{Names: []string{testContainerName}}}

		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(testContainers, nil)
		mockedDockerClient.On("VolumeRemove", mock.Anything, mock.Anything, mock.Anything).Return(testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.RemoveContainer(testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove volume")
	})
	t.Run("RemoveContainer - Failed (failed to remove container)", func(t *testing.T) {
		testContainers := []types.Container{{Names: []string{"/" + testContainerName}, ID: testContainerID}}

		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(testContainers, nil)
		mockedDockerClient.On("VolumeRemove", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).Return(testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.RemoveContainer(testContainerName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove container")
	})
}

func TestStartContainer(t *testing.T) {
	t.Run("StartContainer - Passed", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{State: &types.ContainerState{Running: true}},
		}, nil)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.StartContainer(testAlgorithmName, testContainerID)

		assert.NoError(t, err)
	})
	t.Run("StartContainer - Failed (failed to start container))", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.StartContainer(testAlgorithmName, testContainerID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start container")
	})
	t.Run("StartContainer - Failed (failed to inspect container)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{}, testError)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.StartContainer(testAlgorithmName, testContainerID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to inspect 'TestAlgo' container to validate running state, error: dummy error")
	})
	t.Run("StartContainer - Failed (container is not running)", func(t *testing.T) {
		mockedDockerClient := &dockermocks.MockDockerClient{}
		mockedDockerClient.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockedDockerClient.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{State: &types.ContainerState{Running: false}},
		}, nil)

		containerManagerDocker := &ContainerManagerDocker{
			client: mockedDockerClient,
		}

		err := containerManagerDocker.StartContainer(testAlgorithmName, testContainerID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "'TestAlgo' container started but closed immediately after start")
	})
}

func TestContainerManagerDocker_newModelTar(t *testing.T) {
	t.Run("newModelTar - Passed", func(t *testing.T) {
		createTestFiles(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("failed to clean up test files: %v", err)
			}
		})

		cm := &ContainerManagerDocker{}

		tarBuffer, err := cm.newModelTar(testMlModelDir, testAlgorithmName)

		assert.NoError(t, err)
		assert.NotNil(t, tarBuffer)

		// Verify tar contents
		tarReader := tar.NewReader(tarBuffer)
		filesFound := map[string]bool{}
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			filesFound[header.Name] = true
		}

		assert.True(t, filesFound[testAlgorithmName+"_file1.txt"])
		assert.True(t, filesFound[testAlgorithmName+"_file2.txt"])
		assert.False(t, filesFound["unrelated_file.txt"])
	})
}
