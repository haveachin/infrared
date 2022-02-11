package infrared

import (
	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/multierr"
)

type PluginAPI interface {
	EventBus() event.Bus
	Logger() logr.Logger
}

type Plugin interface {
	Name() string
	Version() string
	Enable(PluginAPI) error
	Disable() error
}

type PluginManager struct {
	Proxies []Proxy
	Plugins []Plugin
	Log     logr.Logger
}

func (pm PluginManager) EventBus() event.Bus {
	return event.DefaultBus
}

func (pm PluginManager) Logger() logr.Logger {
	return pm.Log
}

func (pm PluginManager) EnablePlugins() error {
	var result error
	for _, p := range pm.Plugins {
		pm.Log.Info("loading plugin",
			"pluginName", p.Name(),
			"pluginVersion", p.Version(),
		)
		if err := p.Enable(&pm); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}

func (pm PluginManager) DisablePlugins() error {
	var result error
	for _, p := range pm.Plugins {
		if err := p.Disable(); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}
