package main

import (
	"bytes"
	_ "embed"
	"errors"
	"log"
	"os"

	"github.com/haveachin/infrared"
	"github.com/spf13/viper"
)

const configPath = "config.yml"

//go:embed config.yml
var defaultConfig []byte

func init() {
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(configPath, defaultConfig, 0777); err != nil {
				log.Fatal(err)
			}
			if err := viper.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

func loadConfig(configPath string) (*viper.Viper, error) {
	vpr := viper.New()
	vpr.AddConfigPath(configPath)
	if err := vpr.ReadInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.WriteFile(configPath, defaultConfig, 0777)
		}
	}
	if err := vpr.MergeConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		return nil, err
	}
	if err := vpr.SafeWriteConfig(); err != nil {
		return nil, err
	}
	return vpr, nil
}

func loadGateways() []infrared.Gateway {
	return nil
}
