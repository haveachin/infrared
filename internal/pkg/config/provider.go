package config

import (
	"io/fs"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type provider interface {
	mergeConfigs(v *viper.Viper) error
	close()
}

type fileProvider struct {
	dir      string
	onChange func()
	logger   *zap.Logger
	watcher  *fsnotify.Watcher
}

func (p fileProvider) mergeConfigs(v *viper.Viper) error {
	return filepath.Walk(p.dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		vpr := viper.New()
		vpr.SetConfigFile(path)
		if err := vpr.ReadInConfig(); err != nil {
			p.logger.Error("Failed to read config",
				zap.Error(err),
				zap.String("configPath", path),
			)
			return nil
		}

		return v.MergeConfigMap(vpr.AllSettings())
	})
}

func (p *fileProvider) watch() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	p.watcher = w

	if err := w.Add(p.dir); err != nil {
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
				p.onChange()
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}

			p.logger.Error("Error while watching directory",
				zap.Error(err),
				zap.String("dir", p.dir),
			)
		}
	}
}

func (p fileProvider) close() {
	p.watcher.Close()
}

type dockerProvider struct{}
