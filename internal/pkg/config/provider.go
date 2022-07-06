package config

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
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
	mu       sync.Mutex
}

func (p *fileProvider) mergeConfigs(v *viper.Viper) error {
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
	p.mu.Lock()
	defer p.mu.Unlock()

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

func (p *fileProvider) close() {
	if p.watcher != nil {
		p.watcher.Close()
	}
}

type dockerProvider struct {
	client        *client.Client
	network       string
	clientTimeout time.Duration
	labelPrefix   string
	onChange      func()
	logger        *zap.Logger
}

func (p dockerProvider) mergeConfigs(v *viper.Viper) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.clientTimeout)
	defer cancel()
	containers, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "network",
			Value: p.network,
		}),
	})
	if err != nil {
		return err
	}

	vpr := viper.New()
	for _, container := range containers {
		for key, value := range container.Labels {
			if !strings.HasPrefix(key, p.labelPrefix) {
				continue
			}

			key = strings.TrimPrefix(key, p.labelPrefix)
			if strings.HasPrefix(value, "[") {
				value = strings.Trim(value, "[]")
				vpr.Set(key, strings.Split(value, ","))
			} else {
				vpr.Set(key, value)
			}
		}
	}
	return v.MergeConfigMap(vpr.AllSettings())
}

func (p *dockerProvider) close() {
	p.client.Close()
}
