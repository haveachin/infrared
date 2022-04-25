package config

import (
	"github.com/haveachin/infrared/internal/app/infrared"
)

type Provider interface {
	Configs() ([]infrared.ProxyConfig, error)
	OnConfigChange(fn func(cfgs []infrared.ProxyConfig))
}

type FileProvider struct{}

type DockerProvider struct{}
