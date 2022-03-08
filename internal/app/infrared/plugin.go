package infrared

import (
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type PluginAPI interface {
	EventBus() event.Bus
	Logger() *zap.Logger
}

type Plugin interface {
	Name() string
	Version() string
	Enable(PluginAPI) error
	Disable() error
}

type pluginAPI struct {
	eventBus event.Bus
	logger   *zap.Logger
}

func (api *pluginAPI) EventBus() event.Bus {
	return api.eventBus
}

func (api *pluginAPI) Logger() *zap.Logger {
	return api.logger
}

type PluginManager struct {
	Plugins  []Plugin
	Log      *zap.Logger
	EventBus event.Bus
}

func (pm PluginManager) EnablePlugins() error {
	if pm.EventBus == nil {
		pm.EventBus = event.DefaultBus
	}

	var result error
	for _, p := range pm.Plugins {
		pluginLogger := pm.Log.With(
			zap.String("pluginName", p.Name()),
			zap.String("pluginVersion", p.Version()),
		)
		api := pluginAPI{
			eventBus: pm.EventBus,
			logger:   pluginLogger,
		}

		pluginLogger.Info("loading plugin")
		if err := p.Enable(&api); err != nil {
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
