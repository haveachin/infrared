package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type DockerConfig struct {
	ClientTimeout time.Duration
	LabelPrefix   string
	Endpoint      string
	Network       string
	Watch         bool
}

type docker struct {
	DockerConfig
	client *client.Client
}

func NewDocker(cfg DockerConfig) Provider {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil
	}

	return &docker{
		DockerConfig: cfg,
		client:       cli,
	}
}

func (p docker) Read() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ClientTimeout)
	defer cancel()
	containers, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "network",
			Value: p.Network,
		}),
	})
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	for _, container := range containers {
		for key, value := range container.Labels {
			if !strings.HasPrefix(key, p.LabelPrefix) {
				continue
			}

			key = strings.TrimPrefix(key, fmt.Sprintf("%s.", p.LabelPrefix))
			if strings.HasPrefix(value, "[") {
				value = strings.Trim(value, "[]")
				data[key] = strings.Split(value, ",")
			} else {
				data[key] = value
			}
		}
	}

	return data, nil
}

func (p docker) Watch(onChange func()) error {
	return nil
}

func (p docker) Close() error {
	if p.client != nil {
		if err := p.client.Close(); err != nil {
			return err
		}
	}
	return nil
}
