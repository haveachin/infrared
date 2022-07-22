package config

import (
	"sync"

	"github.com/haveachin/infrared/internal/pkg/config/provider"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type baseConfig struct {
	Providers struct {
		Docker provider.DockerConfig `json:"docker" yaml:"docker"`
		File   provider.FileConfig   `json:"file" yaml:"file"`
	} `json:"providers" yaml:"providers"`
	Watch bool `json:"watch" yaml:"watch"`
}

type config struct {
	baseConfig
	logger *zap.Logger

	dataChan chan provider.Data
	onChange OnChange

	mu           sync.RWMutex
	providerData map[provider.Type]map[string]interface{}
}

type OnChange func(newConfig map[string]interface{})

type Config interface {
	Read() (map[string]interface{}, error)
}

func New(path string, onChange OnChange, logger *zap.Logger) (Config, error) {
	var configMap map[string]interface{}
	if err := provider.ReadConfigFile(path, &configMap); err != nil {
		return nil, err
	}

	var providerCfg baseConfig
	if err := provider.ReadConfigFile(path, &providerCfg); err != nil {
		return nil, err
	}

	if onChange == nil {
		onChange = func(map[string]interface{}) {}
	}

	cfg := config{
		baseConfig: providerCfg,
		logger:     logger,
		dataChan:   make(chan provider.Data),
		onChange:   onChange,
		providerData: map[provider.Type]map[string]interface{}{
			provider.ConfigType: configMap,
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
	for data := range c.dataChan {
		c.mu.Lock()
		c.providerData[data.Type] = data.Config
		c.mu.Unlock()

		c.logger.Info("config changed",
			zap.String("provider", data.Type.String()),
		)

		cfg, err := c.Read()
		if err != nil {
			c.logger.Error("failed to read config",
				zap.Error(err),
			)
		}

		c.onChange(cfg)
	}
}

func (c *config) Read() (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgData := map[string]interface{}{}
	for _, provData := range c.providerData {
		if err := mergo.Merge(&cfgData, provData, mergo.WithOverride); err != nil {
			return nil, err
		}
	}
	return cfgData, nil
}

func Unmarshal(cfg interface{}, v any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     v,
		DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
	})
	if err != nil {
		return err
	}

	return decoder.Decode(cfg)
}
