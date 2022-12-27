package infrared

import (
	"errors"
	"net"
	"regexp"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

var (
	ErrPluginViaConfigDisabled = errors.New("plugin was disabled via config")
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
	PlayerCount(edition Edition) int
}

type PluginAPI interface {
	API

	EventBus() event.Bus
	Logger() *zap.Logger
}

type Plugin interface {
	Name() string
	Version() string
	Init()
	Load(cfg map[string]any) error
	Reload(cfg map[string]any) error
	Enable(api PluginAPI) error
	Disable() error
}

type pluginAPI struct {
	eventBus      event.Bus
	logger        *zap.Logger
	proxies       map[Edition]Proxy
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

func (api pluginAPI) PlayerCount(edition Edition) int {
	return api.proxies[edition].PlayerCount()
}

type PluginState int

const (
	PluginStateDisabled PluginState = iota
	PluginStateLoaded
	PluginStateEnabled
)

type PluginManager struct {
	Proxies  map[Edition]Proxy
	Logger   *zap.Logger
	EventBus event.Bus

	plugins map[Plugin]PluginState
}

func (pm PluginManager) pluginLogger(p Plugin) *zap.Logger {
	return pm.Logger.With(
		zap.String("pluginName", p.Name()),
		zap.String("pluginVersion", p.Version()),
	)
}

func (pm *PluginManager) RegisterPlugin(p Plugin) {
	if pm.plugins == nil {
		pm.plugins = map[Plugin]PluginState{}
	}

	p.Init()
	pm.plugins[p] = PluginStateDisabled
}

func (pm PluginManager) LoadPlugins(cfg map[string]any) {
	for p := range pm.plugins {
		pm.loadPlugin(p, cfg)
	}
}

func (pm PluginManager) loadPlugin(p Plugin, cfg map[string]any) {
	if err := p.Load(cfg); err != nil {
		logger := pm.pluginLogger(p)
		if errors.Is(err, ErrPluginViaConfigDisabled) {
			logger.Debug("Plugin was disabled via config")
		} else {
			logger.Error("failed to load plugin",
				zap.Error(err),
			)
		}
		return
	}

	pm.plugins[p] = PluginStateLoaded
}

func (pm *PluginManager) ReloadPlugins(cfg map[string]any) {
	for p := range pm.plugins {
		pm.reloadPlugin(p, cfg)
	}
}

func (pm *PluginManager) reloadPlugin(p Plugin, cfg map[string]any) {
	logger := pm.pluginLogger(p)
	if err := p.Reload(cfg); err != nil {
		if errors.Is(err, ErrPluginViaConfigDisabled) {
			if pm.isPluginEnabled(p) {
				logger.Debug("disabling plugin via reload")
				pm.disablePlugin(p)
			} else {
				logger.Debug("No change in plugin state")
			}
			return
		}

		logger.Error("failed to reload plugin",
			zap.Error(err),
		)
		return
	}

	if pm.isPluginEnabled(p) {
		return
	}

	logger.Debug("enabling plugin via reload")
	pm.loadPlugin(p, cfg)
	pm.enablePlugin(p)
}

func (pm *PluginManager) isPluginLoaded(p Plugin) bool {
	s, ok := pm.plugins[p]
	return ok && s == PluginStateLoaded
}

func (pm *PluginManager) isPluginEnabled(p Plugin) bool {
	s, ok := pm.plugins[p]
	return ok && s == PluginStateEnabled
}

func (pm *PluginManager) EnablePlugins() {
	for p := range pm.plugins {
		pm.enablePlugin(p)
	}
}

func (pm *PluginManager) enablePlugin(p Plugin) {
	if !pm.isPluginLoaded(p) {
		return
	}

	logger := pm.pluginLogger(p)
	api := pluginAPI{
		proxies:       pm.Proxies,
		pluginManager: *pm,
		eventBus:      pm.EventBus,
		logger:        logger,
	}

	if err := p.Enable(&api); err != nil {
		logger.Error("Failed to enable plugin",
			zap.Error(err),
		)
	}
	pm.plugins[p] = PluginStateEnabled
	logger.Info("enabled plugin")
}

func (pm *PluginManager) DisablePlugins() {
	for p := range pm.plugins {
		pm.disablePlugin(p)
	}
}

func (pm *PluginManager) disablePlugin(p Plugin) {
	logger := pm.pluginLogger(p)
	if err := p.Disable(); err != nil {
		logger.Error("failed to disable plugin",
			zap.Error(err),
		)
	}
	logger.Info("disabling plugin")
	pm.plugins[p] = PluginStateDisabled
}
