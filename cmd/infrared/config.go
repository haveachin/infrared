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
	viper.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err := viper.MergeInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

type gatewayConfig struct {
}

func loadGateways() []infrared.Gateway {
	return nil
}
