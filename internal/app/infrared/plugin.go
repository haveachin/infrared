package infrared

import (
	"sync"

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

type pluginManagerAPI struct {
	mu sync.RWMutex
	pm PluginManager
}

func (api *pluginManagerAPI) EventBus() event.Bus {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.pm.EventBus
}

func (api *pluginManagerAPI) Logger() logr.Logger {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.pm.Log
}

type PluginManager struct {
	Proxies  []Proxy
	Plugins  []Plugin
	Log      logr.Logger
	EventBus event.Bus
}

func (pm PluginManager) EnablePlugins() error {
	if pm.EventBus == nil {
		pm.EventBus = event.DefaultBus
	}
	api := &pluginManagerAPI{pm: pm}

	var result error
	for _, p := range pm.Plugins {
		pm.Log.Info("loading plugin",
			"pluginName", p.Name(),
			"pluginVersion", p.Version(),
		)
		if err := p.Enable(api); err != nil {
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
