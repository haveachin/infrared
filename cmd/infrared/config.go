package main

import (
	"errors"
	"io"
	"os"

	"github.com/haveachin/infrared/configs"
	ir "github.com/haveachin/infrared/pkg/infrared"
	"gopkg.in/yaml.v3"
)

func loadConfig() (ir.Config, error) {
	var f io.ReadCloser
	f, err := os.Open(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return createDefaultConfigFile()
	} else if err != nil {
		return ir.Config{}, err
	}
	defer f.Close()

	cfg := ir.DefaultConfig()
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return ir.Config{}, err
	}

	return cfg, nil
}

func createDefaultConfigFile() (ir.Config, error) {
	bb := configs.DefaultInfraredConfig
	if err := os.WriteFile(configPath, bb, 0666); err != nil {
		return ir.Config{}, err
	}

	cfg := ir.DefaultConfig()
	if err := yaml.Unmarshal(bb, &cfg); err != nil {
		return ir.Config{}, err
	}

	return cfg, nil
}

func loadServerConfigs(path string) ([]ir.ServerConfig, error)
