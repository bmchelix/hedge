package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"hedge/common/errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	edgexNetwork = "edgex_edgex-network"
	modelVolume  = "edgex_hedge-ml-edge-models"
)

type ContainerManager interface {
	CreatePredicationContainer(imagePath, algorithmName string, portNo int64, containerName string) (string, error)
	CreateTrainingContainer(imagePath, name, trainingZip string) (string, error)
	StartContainer(name, containerID string) error
	RunTrainingContainer(name, containerID string) error
	// RemoveContainer removes the container by ContainerName, also removes the volume referred by the container
	RemoveContainer(ContainerName string) error
	RemoveContainerById(containerID string) error
	CopyModelToContainer(containerID, name string) error
}

// DockerClient abstracts the methods required from the Docker client
type DockerClient interface {
	ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	ImagePull(
		ctx context.Context,
		refStr string,
		options image.PullOptions,
	) (io.ReadCloser, error)
	VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error)
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		platform *ocispec.Platform,
		containerName string,
	) (container.CreateResponse, error) // ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	CopyToContainer(
		ctx context.Context,
		containerID, dstPath string,
		content io.Reader,
		options container.CopyToContainerOptions,
	) error
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
}

type ContainerManagerDocker struct {
	client              DockerClient
	modelDir            string
	registry            string
	registryUser        string
	registryPassword    string
	exposeContainerPort bool
	lc                  logger.LoggingClient
}

func NewDockerContainerManager(
	service interfaces.ApplicationService,
	jobDir, registry, registryUser, registryPassword string,
	exposeContainerPort bool,
) (ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to create docker client: %s", err),
		)
	}

	return &ContainerManagerDocker{
		client:              cli,
		modelDir:            jobDir,
		registry:            registry,
		registryUser:        registryUser,
		registryPassword:    registryPassword,
		exposeContainerPort: exposeContainerPort,
		lc:                  service.LoggingClient(),
	}, nil
}

// CreatePredicationContainer creates a new docker predication container and tracks it in Redis
func (cm *ContainerManagerDocker) CreatePredicationContainer(image, algorithmName string, portNo int64,
	containerName string) (string, error) {

	imagePath := filepath.Join(cm.registry, image)
	if err := cm.pullImage(imagePath); err != nil {
		return "", err
	}

	// Create a named volume
	volumeName := strings.ToLower(algorithmName)
	vol, err := cm.client.VolumeCreate(context.Background(), volume.CreateOptions{
		Name: fmt.Sprintf("%s_%s", modelVolume, volumeName),
	})
	if err != nil {
		return "", errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to create volume: %s", err),
		)
	}

	if portNo == 0 {
		portNo = 80
	}

	containerConfig := &container.Config{
		Image: imagePath,
		Env: []string{
			fmt.Sprintf("OUTPUTDIR=%s", cm.modelDir),
			fmt.Sprintf("MODELDIR=%s", cm.modelDir),
			"LOCAL=False",
			fmt.Sprintf("PORT=%d", portNo),
		},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{fmt.Sprintf("%s:%s", vol.Name, cm.modelDir)},
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure,
			MaximumRetryCount: 2},
	}

	if cm.exposeContainerPort {
		portStr := fmt.Sprintf("%d", portNo)
		// var portSet nat.PortSet
		containerConfig.ExposedPorts = map[nat.Port]struct{}{nat.Port(portStr): {}}
		hostConfig.PublishAllPorts = true
		hostConfig.PortBindings = nat.PortMap{nat.Port(portStr): []nat.PortBinding{{
			HostIP:   "0.0.0.0",
			HostPort: portStr,
		},
		}}
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			edgexNetwork: {
				NetworkID: edgexNetwork,
				Aliases:   []string{edgexNetwork},
			},
		},
	}

	resp, err := cm.client.ContainerCreate(
		context.Background(),
		containerConfig,
		hostConfig,
		networkingConfig,
		nil,
		containerName,
	)
	if err != nil {
		return "", errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to create image: %s", err),
		)
	}

	return resp.ID, nil
}

// CreateTrainingContainer Creates a new Docker container and tracks it in Redis
func (cm *ContainerManagerDocker) CreateTrainingContainer(image, containerName, trainingZip string) (string, error) {
	modelDirPath := path.Join(cm.modelDir, filepath.Dir(trainingZip))
	_, modelPathErr := os.Stat(modelDirPath)
	if os.IsNotExist(modelPathErr) {
		return "", errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("The Model directory does not exist or is not valid.: %s", modelDirPath),
		)
	}

	trainingFilePath := path.Join(cm.modelDir, trainingZip)
	_, trgFilePathErr := os.Stat(trainingFilePath)
	if os.IsNotExist(trgFilePathErr) {
		return "", errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("The training file zip does not exist or is not valid.: %s", trainingFilePath),
		)
	}

	imagePath := filepath.Join(cm.registry, image)
	if err := cm.pullImage(imagePath); err != nil {
		return "", err
	}

	// Create a named volume
	//var vol volume.Volume
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: cm.modelDir,
			Target: cm.modelDir,
		},
	}

	containerConfig := &container.Config{
		Image: imagePath,
		Env: []string{
			fmt.Sprintf("MODELDIR=%s", modelDirPath),
			fmt.Sprintf("TRAINING_FILE_ID=%s", trainingFilePath),
			"LOCAL=True",
		},
	}

	hostConfig := &container.HostConfig{
		Mounts:     mounts,
		Privileged: true,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure,
			MaximumRetryCount: 2},
	}

	resp, err := cm.client.ContainerCreate(
		context.Background(),
		containerConfig,
		hostConfig,
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return "", errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to create image: %s", err),
		)
	}

	return resp.ID, nil
}

// StartContainer starts a Docker container and updates its state in Redis
func (cm *ContainerManagerDocker) StartContainer(algorithmName, containerID string) error {
	if err := cm.client.ContainerStart(context.Background(), containerID, container.StartOptions{}); err != nil {
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to start container for %s error: %s", algorithmName, err),
		)
	}

	info, err := cm.client.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf(
				"failed to inspect '%s' container to validate running state, error: %s",
				algorithmName,
				err,
			),
		)
	}

	if !info.State.Running {
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("'%s' container started but closed immediately after start", algorithmName),
		)
	}

	return nil
}

// RunTrainingContainer starts a Docker container and updates its state in Redis
func (cm *ContainerManagerDocker) RunTrainingContainer(name, containerID string) error {
	// Create command to start the container
	cmd := exec.Command("docker", "start", containerID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	cm.lc.Infof("starting container: %s", containerID)
	if err := cmd.Run(); err != nil {
		cm.lc.Errorf("failed to start '%s' container for training, error: %s", name, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf(
				"failed to inspect '%s' container to validate running state",
				name,
			),
		)
	}

	// Attach to logs,
	// to run locally on MAC without docker replace docker by /usr/local/bin/docker
	//logsCmd := exec.Command("/usr/local/bin/docker", "logs", "-f", containerID)
	//logsCmd := exec.Command("docker", "logs", "-f", containerID)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer cancel()
	logsCmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerID)
	logsCmd.Stdout = os.Stdout
	logsCmd.Stderr = os.Stderr

	if err := logsCmd.Run(); err != nil {
		cm.lc.Errorf("failed to get logs for '%s' container in training, error: %s", name, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf(
				"failed to get logs '%s' container to validate training",
				name,
			),
		)
	}

	// Wait for container to exit
	// to run locally on MAC without docker replace docker by /usr/local/bin/docker
	waitCmd := exec.Command("docker", "wait", containerID)
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr

	if err := waitCmd.Run(); err != nil {
		cm.lc.Errorf("failed to waiting on '%s' container to finsih, error: %s", name, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf(
				"failed to waiting on '%s' container to finsih",
				name,
			),
		)
	}

	// Ensure logs command finishes
	err := logsCmd.Wait()
	/*	defer func(containerID string) {
		cm.lc.Errorf("about to remove the container: %s", containerID)
		err := cm.RemoveContainer(containerID)
		if err != nil {
			cm.lc.Errorf("failed to remove '%s' container after successful training error: %s", name, err)
		}
	}(containerID)*/

	info, err := cm.client.ContainerInspect(context.Background(), containerID)
	if err != nil {
		cm.lc.Errorf("failed to inspect '%s' container to validate running state, error: %s", name, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf(
				"failed to inspect '%s' container to validate running state",
				name,
			),
		)
	}

	if info.State.ExitCode != 0 {
		cm.lc.Errorf("'%s' container exited with failure, exit code: %d, error: %s", name, info.State.ExitCode, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("'%s' container exit code none success, exit code: %d", name, info.State.ExitCode),
		)
	}

	cm.lc.Infof("finished training '%s' container: %s status: 'SUCCESS'", name, containerID)
	return nil
}

func (cm *ContainerManagerDocker) RemoveContainer(containerName string) error {
	removeContainer := func(containerID string) error {
		if err := cm.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			return errors.NewCommonHedgeError(
				errors.ErrorTypeServerError,
				fmt.Sprintf("failed to remove container '%s', error: %s", containerID, err),
			)
		}

		return nil
	}

	containers, err := cm.client.ContainerList(
		context.Background(),
		container.ListOptions{All: true},
	)
	if err != nil {
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to list containers for %s: %s", containerName, err),
		)
	}

	for _, cont := range containers {
		for _, name := range cont.Names {
			if !strings.HasPrefix(containerName, "/") {
				containerName = "/" + containerName
			}
			if name == containerName {
				if err = removeContainer(cont.ID); err != nil {
					return err
				}
				break
			}
		}
	}

	// Remove the named volume
	volumeName := fmt.Sprintf("%s_%s", modelVolume, containerName)
	if err = cm.client.VolumeRemove(context.Background(), volumeName, true); err != nil {
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to remove volume %s: %v", volumeName, err),
		)
	}

	return nil
}

func (cm *ContainerManagerDocker) RemoveContainerById(containerID string) error {

	cm.lc.Infof("about to delete the container with Id:%s", containerID)
	if err := cm.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
		cm.lc.Infof("failed to remove training container with Id:%s, error: %v", containerID, err)
		return errors.NewCommonHedgeError(
			errors.ErrorTypeServerError,
			fmt.Sprintf("failed to remove container '%s', error: %s", containerID, err),
		)
	} else {
		cm.lc.Infof("container with Id:%s successfully removed", containerID)
	}

	return nil
}

func (cm *ContainerManagerDocker) CopyModelToContainer(containerID, algorithmName string) error {
	buf, err := cm.newModelTar(cm.modelDir, algorithmName)
	if err != nil {
		return fmt.Errorf("failed to tar directory: %v", err)
	}

	// Create the copy options
	options := container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	// Copy the tar buffer to the container
	err = cm.client.CopyToContainer(context.Background(), containerID, cm.modelDir, buf, options)
	if err != nil {
		return fmt.Errorf("failed to copy to container: %v", err)
	}

	return nil
}

func (cm *ContainerManagerDocker) pullImage(imagePath string) error {
	// check if the image exists on local
	_, _, err := cm.client.ImageInspectWithRaw(context.Background(), imagePath)
	if err != nil {
		switch client.IsErrNotFound(err) {
		case true:
			authConfig := registry.AuthConfig{
				Username: cm.registryUser,
				Password: cm.registryPassword,
			}
			encodedJSON, err := json.Marshal(authConfig)
			if err != nil {
				return errors.NewCommonHedgeError(
					errors.ErrorTypeServerError,
					fmt.Sprintf("failed to marshal auth config: %s", err),
				)
			}
			authStr := base64.URLEncoding.EncodeToString(encodedJSON)
			pullResponse, err := cm.client.ImagePull(
				context.Background(),
				imagePath,
				image.PullOptions{RegistryAuth: authStr},
			)
			if err != nil {
				return errors.NewCommonHedgeError(
					errors.ErrorTypeServerError,
					fmt.Sprintf("failed to pull image: %s", err),
				)
			}
			defer func(pullResponse io.ReadCloser) {
				err := pullResponse.Close()
				if err != nil {
					cm.lc.Errorf("failed to close pull response in defer pullResponse.close(): %v", err)
				}
			}(pullResponse)

			// read the response to ensure the image pull is complete
			_, err = io.Copy(os.Stdout, pullResponse)
			if err != nil {
				return errors.NewCommonHedgeError(
					errors.ErrorTypeServerError,
					fmt.Sprintf("failed to read image pull response: %s", err),
				)
			}
		default:
			return errors.NewCommonHedgeError(
				errors.ErrorTypeServerError,
				fmt.Sprintf("failed to inspect image: %s", err),
			)
		}
	}
	return nil
}

func (cm *ContainerManagerDocker) newModelTar(
	source string,
	algorithm string,
) (*bytes.Buffer, error) {
	// Create a buffer to hold the tar archive
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func(tw *tar.Writer) {
		err := tw.Close()
		if err != nil {
			cm.lc.Errorf("failed to close tar writer in defer tw.Close(): %v", err)
		}
	}(tw)

	// Walk the source directory
	err := filepath.Walk(source, func(filePath string, info os.FileInfo, err error) error {
		if !strings.Contains(filePath, algorithm) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name, _ = filepath.Rel(source, filePath)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer func(file *os.File) {
				err := file.Close()
				if err != nil {
					cm.lc.Errorf("failed to close file in defer file.Close(): %v", err)
				}
			}(file)

			_, err = io.Copy(tw, file)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return buf, nil
}
