package main

import (
	"bytes"
	"log"

	_ "embed"

	"github.com/spf13/viper"
)

//go:embed config.yml
var defaultConfig []byte

func init() {
	configPath = envString(configPathEnv, configPath)

	viper.SetConfigFile(configPath)
	if err := viper.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		log.Fatal(err)
	}

	if err := viper.SafeWriteConfigAs(configPath); err == nil {
		return
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
}
