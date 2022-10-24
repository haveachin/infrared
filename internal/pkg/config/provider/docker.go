package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

type DockerConfig struct {
	ClientTimeout time.Duration `json:"clientTimeout" yaml:"clientTimeout"`
	LabelPrefix   string        `json:"labelPrefix" yaml:"labelPrefix"`
	Endpoint      string        `json:"endpoint" yaml:"endpoint"`
	Network       string        `json:"network" yaml:"network"`
	Watch         bool          `json:"watch" yaml:"watch"`
}

type docker struct {
	DockerConfig
	client *client.Client
	logger *zap.Logger
}

func NewDocker(cfg DockerConfig, logger *zap.Logger) Provider {
	return &docker{
		DockerConfig: cfg,
		logger:       logger,
	}
}

func (p *docker) Provide(dataCh chan<- Data) (Data, error) {
	if p.Endpoint == "" {
		return Data{}, nil
	}

	if p.Endpoint != "unix:///var/run/docker.sock" {
		return Data{}, errors.New("unsupported endpoint")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return Data{}, err
	}
	p.client = cli

	data, err := p.readConfigData()
	if err != nil {
		return Data{}, err
	}

	if p.Watch {
		go func() {
			if err := p.watch(dataCh); err != nil {
				p.logger.Error("failed while watching provider",
					zap.Error(err),
					zap.String("provider", data.Type.String()),
				)
			}
		}()
	}

	return data, nil
}

func (p docker) readConfigData() (Data, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ClientTimeout)
	defer cancel()
	containers, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "network",
			Value: p.Network,
		}),
	})
	if err != nil {
		return Data{}, err
	}

	data := map[string]any{}
	for _, container := range containers {
		for key, value := range container.Labels {
			if !strings.HasPrefix(key, p.LabelPrefix) {
				continue
			}

			key = strings.TrimPrefix(key, p.LabelPrefix)

			if strings.HasPrefix(value, "[") {
				value = strings.Trim(value, "[]")
				setNestedValue(data, key, strings.Split(value, ","))
			} else {
				setNestedValue(data, key, value)
			}
		}
	}

	return Data{
		Type:   DockerType,
		Config: data,
	}, nil
}

func setNestedValue(m map[string]any, nestedKey string, value any) {
	keys := strings.Split(nestedKey, ".")
	keyPath := keys[:len(keys)-1]
	for _, key := range keyPath {
		_, ok := m[key].(map[string]any)
		if !ok {
			m[key] = map[string]any{}
		}
		m = m[key].(map[string]any)
	}
	m[keys[len(keys)-1]] = value
}

func (p docker) watch(dataCh chan<- Data) error {
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
