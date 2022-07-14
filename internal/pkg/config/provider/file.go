package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"sync"

	"github.com/df-mc/atomic"
	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/pkg/maps"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Directory string
	Watch     bool
}

type file struct {
	FileConfig
	logger  *zap.Logger
	watcher *atomic.Value[*fsnotify.Watcher]
	mu      sync.Mutex
}

func NewFile(cfg FileConfig) Provider {
	return &file{
		FileConfig: cfg,
		watcher:    atomic.NewValue[*fsnotify.Watcher](nil),
	}
}

func (p *file) Provide(dataCh chan<- Data) error {
	data, err := p.readConfigData()
	if err != nil {
		return err
	}
	dataCh <- data

	return p.watch(dataCh)
}

func (p *file) watch(dataCh chan<- Data) error {
	p.mu.Lock()
	defer p.mu.Unlock()

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

		configData, err := p.read(path)
		if err != nil {
			p.logger.Error("Failed to read config",
				zap.Error(err),
				zap.String("configPath", path),
			)
			return fmt.Errorf("could not read %s; %v", path, err)
		}

		maps.Merge(cfg, configData)
		return nil
	}

	if err := filepath.Walk(p.Directory, readConfig); err != nil {
		return Data{}, err
	}

	return Data{
		Type:   DockerType,
		Config: cfg,
	}, nil
}

func (p *file) read(filename string) (map[string]interface{}, error) {
	bb, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{}
	ext := filepath.Ext(filename)[1:]
	switch ext {
	case "json":
		if err := json.Unmarshal(bb, data); err != nil {
			return nil, err
		}
	case "yml", "yaml":
		if err := yaml.Unmarshal(bb, data); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported file type")
	}

	return data, nil
}
