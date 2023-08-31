package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/haveachin/infrared/configs"
	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	envVarPrefix = "INFRARED"
)

var (
	configPath = "config.yml"
)

func envVarString(p *string, name string) {
	key := envVarPrefix + "_" + name
	v := os.Getenv(key)
	if v == "" {
		return
	}
	*p = v
}

func initEnvVars() {
	envVarString(&configPath, "CONFIG")
}

func initFlags() {
	pflag.StringVarP(&configPath, "config", "c", configPath, "path to the config file")
	pflag.Parse()
}

func init() {
	initEnvVars()
	initFlags()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("Starting Infrared")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	srv := ir.NewWithConfig(cfg)

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("System is online")

	select {
	case sig := <-sigChan:
		log.Printf("Received %s", sig.String())
	case err := <-errChan:
		if err != nil {
			return err
		}
	}

	return nil
}

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
