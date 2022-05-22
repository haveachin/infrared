package infrared

import (
	"github.com/haveachin/infrared/pkg/event"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type PluginAPI interface {
	EventBus() event.Bus
	Logger() *zap.Logger
}

type Plugin interface {
	Name() string
	Version() string
	Load(v *viper.Viper) error
	Reload(v *viper.Viper) error
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
	Logger   *zap.Logger
	EventBus event.Bus
}

func (pm PluginManager) LoadPlugins(v *viper.Viper) {
	for _, p := range pm.Plugins {
		if err := p.Load(v); err != nil {
			pm.Logger.Error("failed to load plugin",
				zap.Error(err),
				zap.String("pluginName", p.Name()),
				zap.String("pluginVersion", p.Version()),
			)
		}
	}
}

func (pm PluginManager) ReloadPlugins(v *viper.Viper) {
	for _, p := range pm.Plugins {
		if err := p.Load(v); err != nil {
			pm.Logger.Error("failed to reload plugin",
				zap.Error(err),
				zap.String("pluginName", p.Name()),
				zap.String("pluginVersion", p.Version()),
			)
		}
	}
}

func (pm PluginManager) EnablePlugins() {
	if pm.EventBus == nil {
		pm.EventBus = event.DefaultBus
	}

	for _, p := range pm.Plugins {
		pluginLogger := pm.Logger.With(
			zap.String("pluginName", p.Name()),
			zap.String("pluginVersion", p.Version()),
		)
		api := pluginAPI{
			eventBus: pm.EventBus,
			logger:   pluginLogger,
		}

		pluginLogger.Info("enabling plugin")
		if err := p.Enable(&api); err != nil {
			pluginLogger.Error("Failed to enable plugin",
				zap.Error(err),
			)
		}
	}
}

func (pm PluginManager) DisablePlugins() {
	for _, p := range pm.Plugins {
		if err := p.Disable(); err != nil {
			pm.Logger.Error("failed to disable plugin",
				zap.Error(err),
				zap.String("pluginName", p.Name()),
				zap.String("pluginVersion", p.Version()),
			)
		}
	}
}
