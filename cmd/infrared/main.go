package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/haveachin/infrared/pkg/infrared/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

const (
	envVarPrefix = "INFRARED"
)

var (
	configPath = "config.yml"
	workingDir = "."
	proxiesDir = "./proxies"
	logLevel   = "info"
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
	envVarString(&logLevel, "LOG_LEVEL")
}

func initFlags() {
	pflag.StringVarP(&configPath, "config", "c", configPath, "path to the config file")
	pflag.StringVarP(&workingDir, "working-dir", "w", workingDir, "changes the current working directory")
	pflag.StringVarP(&proxiesDir, "proxies-dir", "p", proxiesDir, "path to the proxies directory")
	pflag.StringVarP(&logLevel, "log-level", "l", logLevel, "log level [debug, info, warn, error]")
	pflag.Parse()
}

func initLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})

	var level zerolog.Level
	switch logLevel {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	default:
		log.Warn().
			Str("level", logLevel).
			Msg("Invalid log level; defaulting to info")
	}

	zerolog.SetGlobalLevel(level)
	log.Debug().
		Str("level", logLevel).
		Msg("Log level set")
}

func main() {
	initEnvVars()
	initFlags()
	initLogger()

	log.Info().Msg("Starting Infrared")

	if err := run(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to run")
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
	srv.Logger = log.Logger

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info().Msg("System is online")

	select {
	case sig := <-sigChan:
		log.Info().Msg("Received " + sig.String())
	case err := <-errChan:
		switch {
		case errors.Is(err, ir.ErrNoServers):
			log.Fatal().
				Str("docs", "https://infrared.dev/config/proxies").
				Msg("No proxy configs found; Check the docs")
		case errors.Is(err, ir.ErrNoTrustedCIDRs):
			log.Fatal().
				Str("docs", "https://infrared.dev/features/proxy-protocol#receive-proxy-protocol").
				Msg("Receive PROXY Protocol enabled, but no CIDRs specified; Check the docs")
		default:
			if err != nil {
				return err
			}
		}
	}

	log.Info().Msg("Bye")

	return nil
}
