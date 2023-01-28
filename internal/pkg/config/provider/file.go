package provider

import (
	"bytes"
	"encoding/json"
	"errors"
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
	File  string `mapstructure:"file"`
	Watch bool   `mapstructure:"watch"`
}

type directoryConfig struct {
	Directory string `mapstructure:"directory"`
	Watch     bool   `mapstructure:"watch"`
	// Recursive bool   `json:"recursive" yaml:"recursive"`
}

type FileConfig struct {
	Directories []directoryConfig `mapstructure:"directories"`
	Files       []fileConfig      `mapstructure:"files"`
}

type File struct {
	config    FileConfig
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

	return &File{
		config:    cfg,
		watcher:   atomic.NewValue[*fsnotify.Watcher](nil),
		logger:    logger,
		watchDirs: watchDirs,
	}
}

func (p *File) Provide(dataCh chan<- Data) (Data, error) {
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

// Configs returns a map with the relative file name as key and the config map as value
func (p File) Configs() map[string]map[string]any {
	cfgs := map[string]map[string]any{}
	for _, filePath := range p.filePaths() {
		cfg := map[string]any{}
		if err := ReadConfigFile(filePath, &cfg); err != nil {
			p.logger.Error("failed to read config from file",
				zap.Error(err),
				zap.String("path", filePath),
			)
			continue
		}
		cfgs[filePath] = cfg
	}
	return cfgs
}

func (p *File) watch(dataCh chan<- Data) error {
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

func (p File) Close() error {
	if p.watcher != nil {
		if err := p.watcher.Load().Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p File) filePaths() []string {
	filePaths := []string{}
	for _, dirCfg := range p.config.Directories {
		paths, err := filePathsFromDir(dirCfg.Directory)
		if err != nil {
			p.logger.Error("failed to read config from directory",
				zap.Error(err),
				zap.String("directory", dirCfg.Directory),
			)
		}
		filePaths = append(filePaths, paths...)
	}

	for _, fileDir := range p.config.Files {
		filePaths = append(filePaths, fileDir.File)
	}
	return filePaths
}

func (p File) readConfigData() (Data, error) {
	cfg := map[string]any{}
	for _, filePath := range p.filePaths() {
		fileCfg := map[string]any{}
		if err := ReadConfigFile(filePath, &fileCfg); err != nil {
			p.logger.Error("failed to read config from file",
				zap.Error(err),
				zap.String("path", filePath),
			)
			continue
		}

		if err := mergo.Merge(&cfg, fileCfg, mergo.WithOverride); err != nil {
			p.logger.Error("failed to merge configs",
				zap.Error(err),
				zap.String("path", filePath),
			)
		}
	}

	return Data{
		Type:   FileType,
		Config: cfg,
	}, nil
}

func filePathsFromDir(dir string) ([]string, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return nil, err
		}
	}

	filePaths := []string{}
	readConfig := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink == os.ModeSymlink {
			path, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}

			fi, err := os.Lstat(path)
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return nil
			}
		}

		filePaths = append(filePaths, path)
		return nil
	}

	return filePaths, filepath.WalkDir(dir, readConfig)
}

func ReadConfigFile(name string, v any) error {
	name, err := filepath.EvalSymlinks(name)
	if err != nil {
		return err
	}

	fi, err := os.Lstat(name)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		name, err = os.Readlink(name)
		if err != nil {
			return err
		}
	}

	bb, err := os.ReadFile(name)
	if err != nil {
		return err
	}

	ext := filepath.Ext(name)[1:]
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

func WriteConfigFile(path string, cfg map[string]any) error {
	dir, file := filepath.Split(path)
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return err
		}
	}
	path = filepath.Join(dir, file)

	var bb []byte
	switch ext := filepath.Ext(file)[1:]; ext {
	case "json":
		bb, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
	case "yml", "yaml":
		buf := bytes.NewBuffer([]byte{})
		enc := yaml.NewEncoder(buf)
		enc.SetIndent(2)
		if err := enc.Encode(cfg); err != nil {
			return err
		}
		enc.Close()
		bb = buf.Bytes()
	default:
		return errors.New("unsupported file type")
	}

	return os.WriteFile(path, bb, 0666)
}

func RemoveConfigFile(path string) error {
	dir, file := filepath.Split(path)
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return err
		}
	}
	path = filepath.Join(dir, file)
	return os.Remove(path)
}
