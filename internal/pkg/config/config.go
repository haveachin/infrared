package config

import (
	"reflect"
	"sync"

	"github.com/c2h5oh/datasize"
	"github.com/haveachin/infrared/internal/pkg/config/provider"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type baseConfig struct {
	Providers struct {
		Docker provider.DockerConfig `mapstructure:"docker"`
		File   provider.FileConfig   `mapstructure:"file"`
	} `mapstructure:"providers"`
}

type config struct {
	baseConfig
	logger *zap.Logger

	dataChan chan provider.Data
	onChange OnChange

	mu           sync.RWMutex
	providerData map[provider.Type]map[string]any
}

type OnChange func(newConfig map[string]any)

type Config interface {
	Read() (map[string]any, error)
}

func New(path string, onChange OnChange, logger *zap.Logger) (Config, error) {
	var configMap map[string]any
	if err := provider.ReadConfigFile(path, &configMap); err != nil {
		return nil, err
	}

	var providerCfg baseConfig
	if err := Unmarshal(configMap, &providerCfg); err != nil {
		return nil, err
	}

	if onChange == nil {
		onChange = func(map[string]any) {}
	}

	cfg := config{
		baseConfig: providerCfg,
		logger:     logger,
		dataChan:   make(chan provider.Data),
		onChange:   onChange,
		providerData: map[provider.Type]map[string]any{
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
			logger.Fatal("failed to provide config data",
				zap.Error(err),
			)
			continue
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
			continue
		}
		c.onChange(cfg)
	}
}

func (c *config) Read() (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgData := map[string]any{}
	for _, provData := range c.providerData {
		var err error
		cfgData, err = MergeConfigsMaps(cfgData, provData)
		if err != nil {
			return nil, err
		}
	}
	return cfgData, nil
}

func Unmarshal(cfg any, v any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: v,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			stringToDataSizeHookFunc(),
		),
	})
	if err != nil {
		return err
	}

	return decoder.Decode(cfg)
}

func stringToDataSizeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(datasize.ByteSize(5)) {
			return data, nil
		}

		// Convert it by parsing
		return datasize.ParseString(data.(string))
	}
}
