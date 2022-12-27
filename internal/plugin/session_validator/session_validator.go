package session_validator

import (
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type PluginConfig struct {
	Enable   bool `mapstructure:"enabled"`
	Defaults struct {
	} `mapstructure:"defaults"`
}

type Plugin struct {
	Config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus
}
