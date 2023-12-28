package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/haveachin/infrared/pkg/infrared/config"
	"github.com/spf13/pflag"
)

const (
	envVarPrefix = "INFRARED"
)

var (
	configPath = "config.yml"
	workingDir = "."
	proxiesDir = "./proxies"
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
	envVarString(&workingDir, "WORKING_DIR")
	envVarString(&proxiesDir, "PROXIES_DIR")
}

func initFlags() {
	pflag.StringVarP(&configPath, "config", "c", configPath, "path to the config file")
	pflag.StringVarP(&workingDir, "working-dir", "w", workingDir, "changes the current working directory")
	pflag.StringVarP(&proxiesDir, "proxies-dir", "p", proxiesDir, "path to the proxies directory")
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
	if err := os.Chdir(workingDir); err != nil {
		return err
	}

	if err := createConfigIfNotExist(); err != nil {
		return err
	}

	srv := ir.NewWithConfigProvider(config.FileProvider{
		ConfigPath:  configPath,
		ProxiesPath: proxiesDir,
	})

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
