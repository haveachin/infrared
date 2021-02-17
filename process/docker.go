package process

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type docker struct {
	client        *client.Client
	containerName string
}

// NewDocker create a new docker process that manages a container
func NewDocker(containerName string) (Process, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return docker{
		client:        cli,
		containerName: fmt.Sprintf("/%s", containerName),
	}, nil
}

func (proc docker) Start() error {
	containerID, err := proc.resolveContainerName()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	return proc.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

func (proc docker) Stop() error {
	containerID, err := proc.resolveContainerName()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	return proc.client.ContainerStop(ctx, containerID, nil)
}

func (proc docker) IsRunning() (bool, error) {
	containerID, err := proc.resolveContainerName()
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := proc.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	return info.State.Running, nil
}

func (proc docker) resolveContainerName() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	containers, err := proc.client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if name != proc.containerName {
				continue
			}
			return container.ID, nil
		}
	}

	return "", fmt.Errorf("container with name \"%s\" not found", proc.containerName)
}
