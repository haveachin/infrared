package main

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/haveachin/infrared/configs"
	ir "github.com/haveachin/infrared/pkg/infrared"
	"gopkg.in/yaml.v3"
)

func createConfigIfNotExist() error {
	info, err := os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(proxiesDir, 0755); err != nil {
			return err
		}

		return createDefaultConfigFile()
	}

	if info.IsDir() {
		return errors.New("ir.Config is a directory")
	}

	return nil
}

func createDefaultConfigFile() error {
	bb := configs.DefaultInfraredConfig
	return os.WriteFile(configPath, bb, 0664)
}

func loadConfig() (ir.Config, error) {
	var f io.ReadCloser
	f, err := os.Open(configPath)
	if err != nil {
		return ir.Config{}, err
	}
	defer f.Close()

	cfg := ir.DefaultConfig()
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return ir.Config{}, err
	}

	srvCfgs, err := loadServerConfigs(proxiesDir)
	if err != nil {
		return ir.Config{}, err
	}
	cfg.ServerConfigs = srvCfgs

	return cfg, nil
}

func loadServerConfigs(path string) ([]ir.ServerConfig, error) {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	if err := filepath.WalkDir(path, walkServerDirFunc(&paths)); err != nil {
		return nil, err
	}

	return readAndUnmashalServerConfigs(paths)
}

func walkServerDirFunc(paths *[]string) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink == os.ModeSymlink {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
		}

		*paths = append(*paths, path)
		return nil
	}
}

func readAndUnmashalServerConfigs(paths []string) ([]ir.ServerConfig, error) {
	cfgs := make([]ir.ServerConfig, 0)
	for _, path := range paths {
		cfg, err := readAndUnmashalServerConfig(path)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}

func readAndUnmashalServerConfig(path string) (ir.ServerConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return ir.ServerConfig{}, err
	}
	defer f.Close()

	cfg := ir.ServerConfig{}
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return ir.ServerConfig{}, err
	}

	return cfg, nil
}
