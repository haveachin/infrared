package config

import (
	"sync"

	"github.com/haveachin/infrared/internal/pkg/config/provider"
	"github.com/haveachin/infrared/pkg/maps"
	"go.uber.org/zap"
)

type providerConfig struct {
	Providers struct {
		Docker provider.DockerConfig `json:"docker" yaml:"docker"`
		File   provider.FileConfig   `json:"file" yaml:"file"`
	} `json:"providers" yaml:"providers"`
}

type config struct {
	providerConfig
	logger *zap.Logger

	dataChan chan provider.Data
	onChange OnChange

	mu           sync.RWMutex
	providerData map[provider.Type]map[string]interface{}
}

type OnChange func(newConfig map[string]interface{})

type Config interface {
	Read() map[string]interface{}
}

func New(path string, onChange OnChange, logger *zap.Logger) (Config, error) {
	var configMap map[string]interface{}
	if err := provider.ReadConfigFile(path, &configMap); err != nil {
		return nil, err
	}

	var providerCfg providerConfig
	if err := provider.ReadConfigFile(path, &providerCfg); err != nil {
		return nil, err
	}

	if onChange == nil {
		onChange = func(newConfig map[string]interface{}) {}
	}

	cfg := config{
		providerConfig: providerCfg,
		logger:         logger,
		dataChan:       make(chan provider.Data),
		onChange:       onChange,
		providerData: map[provider.Type]map[string]interface{}{
			provider.BaseType: configMap,
		},
	}

	providers := []provider.Provider{
		provider.NewFile(cfg.Providers.File, logger),
		provider.NewDocker(cfg.Providers.Docker, logger),
	}

	for _, prov := range providers {
		data, err := prov.Provide(cfg.dataChan)
		if err != nil {
			logger.Warn("failed to provide config data",
				zap.Error(err),
			)
		}

		if data.IsNil() {
			continue
		}

		cfg.providerData[data.Type] = data.Config
	}

	go cfg.listenToProviders()
	return &cfg, nil
}

func (c *config) listenToProviders() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for data := range c.dataChan {
		c.providerData[data.Type] = data.Config
		c.logger.Info("config changed",
			zap.String("provider", data.Type.String()),
		)
		c.onChange(c.Read())
	}
}

func (c *config) Read() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgData := map[string]interface{}{}
	for _, provData := range c.providerData {
		maps.Merge(cfgData, provData)
	}
	return cfgData
}
