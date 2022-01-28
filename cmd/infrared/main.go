package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/haveachin/infrared/internal/app/infrared"
	"go.uber.org/zap"
)

const configPathEnv = "INFRARED_CONFIG_PATH"

var configPath = "config.yml"

func envString(name string, value string) string {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	return envString
}

var logger logr.Logger

func init() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to init logger; err: %s", err)
	}
	logger = zapr.NewLogger(zapLog)
}

func main() {
	logger.Info("loading proxy")

	bedrockProxy, err := infrared.NewProxy(&BedrockProxyConfig{})
	if err != nil {
		logger.Error(err, "failed to load proxy")
		return
	}

	javaProxy, err := infrared.NewProxy(&JavaProxyConfig{})
	if err != nil {
		logger.Error(err, "failed to load proxy")
		return
	}

	logger.Info("starting proxy")

	go func() {
		if err := bedrockProxy.Start(logger); err != nil {
			logger.Error(err, "failed to start the proxy")
			os.Exit(1)
		}
	}()

	go func() {
		if err := javaProxy.Start(logger); err != nil {
			logger.Error(err, "failed to start the proxy")
			os.Exit(1)
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
