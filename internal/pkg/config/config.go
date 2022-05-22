package config

import (
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type content struct {
	Providers struct {
		Docker struct {
			Endpoint string
			Network  string
			Watch    bool
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
	content   content
	providers []provider
}

func (c *Config) Load() error {
	if c.Logger == nil {
		c.Logger = zap.NewNop()
	}

	c.v = viper.New()
	c.v.SetConfigFile(c.Path)
	if err := c.v.ReadInConfig(); err != nil {
		return err
	}

	if err := c.v.Unmarshal(&c.content); err != nil {
		return err
	}

	c.providers = []provider{
		c.initFileProvider(),
	}

	return nil
}

func (c *Config) initFileProvider() provider {
	cfg := c.content.Providers.File
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

func (c *Config) ReadConfigs() (*viper.Viper, []infrared.ProxyConfig, error) {
	if c.v == nil {
		if err := c.Load(); err != nil {
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

func (c *Config) Close() {
	for _, p := range c.providers {
		p.close()
	}
}
