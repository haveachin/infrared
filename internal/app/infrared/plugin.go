package infrared

import (
	"net"
	"regexp"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type Player interface {
	Username() string
	Edition() Edition
	GatewayID() string
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Close() error
}

type API interface {
	PlayerByUsername(edition Edition, username string) Player
	Players(edition Edition, usernamePattern string) ([]Player, error)
}

type PluginAPI interface {
	API

	EventBus() event.Bus
	Logger() *zap.Logger
}

type Plugin interface {
	Name() string
	Version() string
	Load(cfg map[string]interface{}) error
	Reload(cfg map[string]interface{}) error
	Enable(api PluginAPI) error
	Disable() error
}

type pluginAPI struct {
	eventBus      event.Bus
	logger        *zap.Logger
	proxies       map[Edition]*Proxy
	pluginManager PluginManager
}

func (api pluginAPI) EventBus() event.Bus {
	return api.eventBus
}

func (api pluginAPI) Logger() *zap.Logger {
	return api.logger
}

func (api pluginAPI) PlayerByUsername(edition Edition, username string) Player {
	if username == "" {
		return nil
	}

	for _, player := range api.proxies[edition].Players() {
		if player.Username() == username {
			return player
		}
	}
	return nil
}

func (api pluginAPI) Players(edition Edition, usernameRegex string) ([]Player, error) {
	players := api.proxies[edition].Players()
	if usernameRegex == "" {
		return players, nil
	}

	pattern, err := regexp.Compile(usernameRegex)
	if err != nil {
		return nil, err
	}

	pp := make([]Player, 0, len(players))
	for _, player := range players {
		if pattern.MatchString(player.Username()) {
			pp = append(pp, player)
		}
	}
	return pp, nil
}

type PluginManager struct {
	Proxies  map[Edition]*Proxy
	Plugins  []Plugin
	Logger   *zap.Logger
	EventBus event.Bus
}

func (pm PluginManager) LoadPlugins(cfg map[string]interface{}) {
	for _, p := range pm.Plugins {
		if err := p.Load(cfg); err != nil {
			pm.Logger.Error("failed to load plugin",
				zap.Error(err),
				zap.String("pluginName", p.Name()),
				zap.String("pluginVersion", p.Version()),
			)
		}
	}
}

func (pm PluginManager) ReloadPlugins(cfg map[string]interface{}) {
	for _, p := range pm.Plugins {
		if err := p.Reload(cfg); err != nil {
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
			proxies:       pm.Proxies,
			pluginManager: pm,
			eventBus:      pm.EventBus,
			logger:        pluginLogger,
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
