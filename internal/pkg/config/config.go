package config

import (
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type config struct {
	Providers struct {
		Docker struct {
			ClientTimeout time.Duration
			LabelPrefix   string
			Endpoint      string
			Network       string
			Watch         bool
		}
		File struct {
			Directory string
			Watch     bool
		}
	}
}

type Config struct {
	Path     string
	Logger   *zap.Logger
	OnChange func(*viper.Viper, []infrared.ProxyConfig)

	v         *viper.Viper
	config    config
	providers []provider
	mu        sync.Mutex
}

func (c *Config) init() error {
	if c.Logger == nil {
		c.Logger = zap.NewNop()
	}

	c.v = viper.New()
	c.v.SetConfigFile(c.Path)
	if err := c.v.ReadInConfig(); err != nil {
		return err
	}

	if err := c.v.Unmarshal(&c.config); err != nil {
		return err
	}

	c.providers = []provider{
		c.initFileProvider(),
	}

	if c.config.Providers.Docker.Endpoint != "" {
		c.providers = append(c.providers, c.initDockerProvider())
	}

	return nil
}

func (c *Config) initFileProvider() provider {
	cfg := c.config.Providers.File
	fileProvider := fileProvider{
		dir:      cfg.Directory,
		onChange: c.onChange,
		logger:   c.Logger,
	}

	if cfg.Watch {
		go func() {
			if err := fileProvider.watch(); err != nil {
				c.Logger.Error("", zap.Error(err))
			}
		}()
	}

	return &fileProvider
}

func (c *Config) initDockerProvider() provider {
	cfg := c.config.Providers.Docker
	// TODO: From URL?
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		c.Logger.Error("", zap.Error(err))
		return nil
	}

	dockerProvider := dockerProvider{
		client:        cli,
		clientTimeout: cfg.ClientTimeout,
		labelPrefix:   cfg.LabelPrefix,
		network:       cfg.Network,
		onChange:      c.onChange,
		logger:        c.Logger,
	}

	if cfg.Watch {
		// TODO: Add network change event?
	}

	return &dockerProvider
}

func (c *Config) ReadConfigs() (*viper.Viper, []infrared.ProxyConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.v == nil {
		if err := c.init(); err != nil {
			return nil, nil, err
		}
	}

	v := viper.New()
	v.MergeConfigMap(c.v.AllSettings())
	for _, p := range c.providers {
		if err := p.mergeConfigs(v); err != nil {
			return nil, nil, err
		}
	}

	return v, []infrared.ProxyConfig{
		java.ProxyConfig{Viper: v},
		bedrock.ProxyConfig{Viper: v},
	}, nil
}

func (c *Config) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range c.providers {
		p.close()
	}
}

func (c *Config) onChange() {
	if c.OnChange == nil {
		return
	}

	v, cfgs, err := c.ReadConfigs()
	if err != nil {
		c.Logger.Error("Failed to read configs",
			zap.Error(err),
		)
	}

	c.OnChange(v, cfgs)
}
