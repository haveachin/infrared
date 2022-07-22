package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"

	"github.com/df-mc/atomic"
	"github.com/fsnotify/fsnotify"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type FileConfig struct {
	Directory string `json:"directory" yaml:"directory"`
	Watch     bool   `json:"watch" yaml:"watch"`
}

type file struct {
	FileConfig
	watcher *atomic.Value[*fsnotify.Watcher]
	logger  *zap.Logger
}

func NewFile(cfg FileConfig, logger *zap.Logger) Provider {
	return &file{
		FileConfig: cfg,
		watcher:    atomic.NewValue[*fsnotify.Watcher](nil),
		logger:     logger,
	}
}

func (p *file) Provide(dataCh chan<- Data) (Data, error) {
	data, err := p.readConfigData()
	if err != nil {
		return Data{}, err
	}

	if p.Watch {
		go func() {
			if err := p.watch(dataCh); err != nil {
				p.logger.Error("failed while watching provider",
					zap.Error(err),
					zap.String("provider", data.Type.String()),
				)
			}
		}()
	}

	return data, nil
}

func (p *file) watch(dataCh chan<- Data) error {
	if p.watcher.Load() != nil {
		return errors.New("already watching")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	p.watcher.Store(w)

	if err := w.Add(p.Directory); err != nil {
		return err
	}

	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				return nil
			}

			if e.Op&fsnotify.Remove == fsnotify.Remove ||
				e.Op&fsnotify.Write == fsnotify.Write ||
				e.Op&fsnotify.Create == fsnotify.Create ||
				e.Op&fsnotify.Rename == fsnotify.Rename ||
				e.Op == fsnotify.Remove {
				data, err := p.readConfigData()
				if err != nil {
					return err
				}
				dataCh <- data
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}

			p.logger.Error("Error while watching directory",
				zap.Error(err),
				zap.String("dir", p.Directory),
			)
		}
	}
}

func (p file) Close() error {
	if p.watcher != nil {
		if err := p.watcher.Load().Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p file) readConfigData() (Data, error) {
	cfg := map[string]interface{}{}
	readConfig := func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		cfgData := map[string]interface{}{}
		if err := ReadConfigFile(path, &cfgData); err != nil {
			p.logger.Error("Failed to read config",
				zap.Error(err),
				zap.String("configPath", path),
			)
			return fmt.Errorf("could not read %s; %v", path, err)
		}

		return mergo.Merge(&cfg, cfgData, mergo.WithOverride)
	}

	if err := filepath.Walk(p.Directory, readConfig); err != nil {
		return Data{}, err
	}

	return Data{
		Type:   FileType,
		Config: cfg,
	}, nil
}

func ReadConfigFile(filename string, v any) error {
	bb, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	ext := filepath.Ext(filename)[1:]
	switch ext {
	case "json":
		if err := json.Unmarshal(bb, v); err != nil {
			return err
		}
	case "yml", "yaml":
		if err := yaml.Unmarshal(bb, v); err != nil {
			return err
		}
	default:
		return errors.New("unsupported file type")
	}

	return nil
}
