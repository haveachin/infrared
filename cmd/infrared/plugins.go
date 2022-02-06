package main

import (
	"errors"
	"log"
	"os"
)

func initPlugins() {
	if err := createDirIfNotExist(pluginsPath); err != nil {
		log.Fatal(err)
	}
}

func createDirIfNotExist(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Mkdir("plugins", 0644)
}
