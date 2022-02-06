package infrared

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/multierr"
)

var ErrInvalidPluginImplementation = errors.New("invalid plugin implementation")

type Plugin interface {
	Name() string
	Version() string
	Enable(logr.Logger, *event.Bus) error
	Disable() error
}

func LoadPluginFromFile(path string) (Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	v, err := p.Lookup("Plugin")
	if err != nil {
		return nil, err
	}

	pv, ok := v.(*Plugin)
	if !ok {
		return nil, ErrInvalidPluginImplementation
	}

	return *pv, nil
}

func LoadPluginsFromDir(path string) ([]Plugin, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var pp []Plugin
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || !strings.HasSuffix(name, ".so") {
			continue
		}

		filePath := filepath.Join(path, name)
		p, err := LoadPluginFromFile(filePath)
		if err != nil {
			return nil, err
		}
		pp = append(pp, p)
	}
	return pp, nil
}

type PluginManager struct {
	Proxies []Proxy
	Plugins []Plugin
	Log     logr.Logger
}

func (pm PluginManager) EnablePlugins() error {
	var result error
	for _, p := range pm.Plugins {
		pm.Log.Info("loading plugin",
			"pluginName", p.Name(),
			"pluginVersion", p.Version(),
		)
		if err := p.Enable(pm.Log, event.DefaultBus); err != nil {
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
