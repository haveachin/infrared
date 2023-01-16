package infrared

import (
	"errors"
	"regexp"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

var (
	ErrPluginViaConfigDisabled = errors.New("plugin was disabled via config")
)

type API interface {
	// PlayerByUsername returns a Player with a specified username and Edition if they are currently being proxied.
	PlayerByUsername(username string, edition Edition) Player

	// Players returns a slice of Player that are currently being proxied filtered by a regular expression.
	// If a edition is specified more than once than the slice of Player will contain duplicates.
	// If no editions are specified this returns an empty slice.
	Players(usernameRegex string, editions ...Edition) ([]Player, error)

	// Returns the amount of players that are currently being proxied with the specified Edition.
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
	logger *zap.Logger
	pm     *PluginManager
}

func (api pluginAPI) EventBus() event.Bus {
	return api.pm.EventBus
}

func (api pluginAPI) Logger() *zap.Logger {
	return api.logger
}

func (api pluginAPI) PlayerByUsername(username string, edition Edition) Player {
	if username == "" {
		return nil
	}

	for _, player := range api.pm.Proxies[edition].Players() {
		if player.Username() == username {
			return player
		}
	}
	return nil
}

func (api pluginAPI) Players(usernameRegex string, editions ...Edition) ([]Player, error) {
	if len(editions) == 0 {
		return []Player{}, nil
	}

	players := []Player{}
	for _, edition := range editions {
		players = append(players, api.pm.Proxies[edition].Players()...)
	}

	if usernameRegex == ".*" {
		return players, nil
	}

	pattern, err := regexp.Compile(usernameRegex)
	if err != nil {
		return nil, err
	}

	pp := []Player{}
	for _, player := range players {
		if pattern.MatchString(player.Username()) {
			pp = append(pp, player)
		}
	}
	return pp, nil
}

func (api pluginAPI) PlayerCount(edition Edition) int {
	return api.pm.Proxies[edition].PlayerCount()
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
	pm.pluginLogger(p).Debug("plugin registered")
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
			logger.Debug("plugin was disabled via config")
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
	for p, s := range pm.plugins {
		if s == PluginStateDisabled {
			pm.loadPlugin(p, cfg)
			return
		}

		pm.reloadPlugin(p, cfg)
	}
}

func (pm *PluginManager) reloadPlugin(p Plugin, cfg map[string]any) {
	logger := pm.pluginLogger(p)
	if err := p.Reload(cfg); err != nil {
		if errors.Is(err, ErrPluginViaConfigDisabled) {
			if pm.isPluginEnabled(p) {
				pm.disablePlugin(p)
				logger.Debug("disabled plugin via reload")
			} else {
				logger.Debug("no change in plugin state")
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

	pm.loadPlugin(p, cfg)
	pm.enablePlugin(p)
	logger.Debug("enabled plugin via reload")
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
	api := &pluginAPI{
		pm:     pm,
		logger: logger,
	}

	if err := p.Enable(api); err != nil {
		logger.Error("failed to enable plugin",
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
	if !pm.isPluginEnabled(p) {
		return
	}

	logger := pm.pluginLogger(p)
	if err := p.Disable(); err != nil {
		logger.Error("failed to disable plugin",
			zap.Error(err),
		)
	}
	pm.plugins[p] = PluginStateDisabled
	logger.Info("disabled plugin")
}
