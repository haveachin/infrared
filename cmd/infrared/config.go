package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"

	_ "embed"

	"github.com/spf13/viper"
)

//go:embed config.default.yml
var defaultConfig []byte

var v *viper.Viper

func initConfig() {
	v = viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		log.Fatal(err)
	}

	if err := createConfigIfNonExist(configPath); err != nil {
		log.Fatal(err)
	}

	if err := v.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
}

func createConfigIfNonExist(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return ioutil.WriteFile(path, defaultConfig, 0644)
}
