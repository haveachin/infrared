package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/df-mc/atomic"
	"github.com/fsnotify/fsnotify"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	File  string `json:"file" yaml:"file"`
	Watch bool   `json:"watch" yaml:"watch"`
}

type directoryConfig struct {
	Directory string `json:"directory" yaml:"directory"`
	Watch     bool   `json:"watch" yaml:"watch"`
	// Recursive bool   `json:"recursive" yaml:"recursive"`
}

type FileConfig struct {
	Directories []directoryConfig `json:"directories" yaml:"directories"`
	Files       []fileConfig      `json:"files" yaml:"files"`
}

type file struct {
	FileConfig
	watcher   *atomic.Value[*fsnotify.Watcher]
	logger    *zap.Logger
	watchDirs []string
}

func NewFile(cfg FileConfig, logger *zap.Logger) Provider {
	watchDirs := []string{}
	for _, dirCfg := range cfg.Directories {
		if dirCfg.Watch {
			watchDirs = append(watchDirs, dirCfg.Directory)
		}
	}

	for _, fileCfg := range cfg.Files {
		if fileCfg.Watch {
			watchDirs = append(watchDirs, fileCfg.File)
		}
	}

	return &file{
		FileConfig: cfg,
		watcher:    atomic.NewValue[*fsnotify.Watcher](nil),
		logger:     logger,
		watchDirs:  watchDirs,
	}
}

func (p *file) Provide(dataCh chan<- Data) (Data, error) {
	data, err := p.readConfigData()
	if err != nil && len(p.watchDirs) <= 0 {
		return Data{}, err
	}

	if len(p.watchDirs) > 0 {
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

	for _, dir := range p.watchDirs {
		if err := w.Add(dir); err != nil {
			return err
		}
	}

	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				p.logger.Debug("Closing file watcher",
					zap.String("cause", "watcher event channel closed"),
				)
				return nil
			}

			if e.Op&fsnotify.Remove == fsnotify.Remove ||
				e.Op&fsnotify.Write == fsnotify.Write ||
				e.Op&fsnotify.Create == fsnotify.Create ||
				e.Op&fsnotify.Rename == fsnotify.Rename ||
				e.Op == fsnotify.Remove {
				data, err := p.readConfigData()
				if err != nil {
					continue
				}
				dataCh <- data
			}
		case err, ok := <-w.Errors:
			if !ok {
				p.logger.Debug("closing file watcher",
					zap.String("cause", "watcher error channel closed"),
				)
				return nil
			}

			p.logger.Error("error while watching directory",
				zap.Error(err),
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
	cfg := map[string]any{}
	for _, dirCfg := range p.Directories {
		if err := readConfigsFromDir(dirCfg.Directory, &cfg); err != nil {
			p.logger.Error("failed to read config from directory",
				zap.Error(err),
				zap.String("directory", dirCfg.Directory),
			)
		}
	}

	for _, fileDir := range p.Files {
		if err := ReadConfigFile(fileDir.File, &cfg); err != nil {
			p.logger.Error("failed to read config from file",
				zap.Error(err),
				zap.String("directory", fileDir.File),
			)
		}
	}

	return Data{
		Type:   FileType,
		Config: cfg,
	}, nil
}

func readConfigsFromDir(dir string, v any) error {
	readConfig := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		cfgData := map[string]any{}
		if err := ReadConfigFile(path, &cfgData); err != nil {
			return fmt.Errorf("could not read %s; %v", path, err)
		}

		return mergo.Merge(v, cfgData, mergo.WithOverride)
	}

	return filepath.Walk(dir, readConfig)
}

func ReadConfigFile(filename string, v any) error {
	bb, err := os.ReadFile(filename)
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
