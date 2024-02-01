package main

import (
	"errors"
	"os"

	"github.com/haveachin/infrared/configs"
)

func createConfigIfNotExist() error {
	info, err := os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		if err = os.Mkdir(proxiesDir, 0755); err != nil {
			return err
		}

		return createDefaultConfigFile()
	} else if err != nil {
		return err
	}

	if info.IsDir() {
		return errors.New("ir.Config is a directory")
	}

	return nil
}

func createDefaultConfigFile() error {
	bb := configs.DefaultInfraredConfig
	return os.WriteFile(configPath, bb, 0600)
}
