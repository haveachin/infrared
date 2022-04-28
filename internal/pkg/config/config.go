package config

import (
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/spf13/viper"
)

type Config interface {
	ReadProxyConfigs() ([]infrared.ProxyConfig, error)
	OnConfigChange(fn func(cfgs []infrared.ProxyConfig))
}

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

func New(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var content content
	if err := v.Unmarshal(&content); err != nil {
		return nil, err
	}

	return &config{
		v:       v,
		content: content,
		providers: []Provider{
			&fileProvider{
				dirPath: content.Providers.File.Directory,
			},
		},
	}, nil
}

type config struct {
	v            *viper.Viper
	content      content
	providers    []Provider
	configChange func(cfgs []infrared.ProxyConfig)
}

func (c config) ReadProxyConfigs() ([]infrared.ProxyConfig, error) {
	v := viper.New()
	v.MergeConfigMap(c.v.AllSettings())
	for _, p := range c.providers {
		if err := p.mergeConfigs(v); err != nil {
			return nil, err
		}
	}

	return []infrared.ProxyConfig{
		java.ProxyConfig{Viper: v},
		bedrock.ProxyConfig{Viper: v},
	}, nil
}

func (c *config) OnConfigChange(fn func(cfgs []infrared.ProxyConfig)) {
	c.configChange = fn
}
