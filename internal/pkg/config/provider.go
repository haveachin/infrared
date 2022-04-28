package config

import (
	"io/fs"
	"path/filepath"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/spf13/viper"
)

type Provider interface {
	mergeConfigs(v *viper.Viper) error
	onConfigChange(fn func(cfgs []infrared.ProxyConfig))
}

type fileProvider struct {
	dirPath        string
	configChange func(cfgs []infrared.ProxyConfig)
}

func (p fileProvider) mergeConfigs(v *viper.Viper) error {
	return filepath.Walk(p.dirPath, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		vpr := viper.New()
		vpr.SetConfigFile(path)
		if err := vpr.ReadInConfig(); err != nil {
			return err
		}

		return v.MergeConfigMap(vpr.AllSettings())
	})
}

func (p fileProvider) onConfigChange(fn func(cfgs []infrared.ProxyConfig)) {

}

type dockerProvider struct{}
