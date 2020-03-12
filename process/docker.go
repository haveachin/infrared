package process

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type dockerProcess struct {
	client      *client.Client
	containerID string
}

// NewDocker create a new docker process that manages a container
func NewDocker(containerID string) (Process, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return dockerProcess{
		client:      cli,
		containerID: containerID,
	}, nil
}

func (proc dockerProcess) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	return proc.client.ContainerStart(ctx, proc.containerID, types.ContainerStartOptions{})
}

func (proc dockerProcess) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	return proc.client.ContainerStop(ctx, proc.containerID, nil)
}

func (proc dockerProcess) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := proc.client.ContainerInspect(ctx, proc.containerID)
	if err != nil {
		return false
	}

	return info.State.Running
}
