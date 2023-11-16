package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/spf13/pflag"
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
	log.Println("Starting Infrared")

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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
