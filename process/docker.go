package process

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type docker struct {
	client      *client.Client
	containerID string
}

// NewDocker create a new docker process that manages a container
func NewDocker(containerID string) (Process, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return docker{
		client:      cli,
		containerID: containerID,
	}, nil
}

func (proc docker) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	return proc.client.ContainerStart(ctx, proc.containerID, types.ContainerStartOptions{})
}

func (proc docker) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	return proc.client.ContainerStop(ctx, proc.containerID, nil)
}
