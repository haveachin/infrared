package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/haveachin/infrared/internal/app/infrared"
	"go.uber.org/zap"
)

const (
	envPrefix      = "INFRARED_"
	envConfigPath  = envPrefix + "CONFIG_PATH"
	envPluginsPath = envPrefix + "PLUGINS_PATH"

	clfConfigPath  = "config-path"
	clfPluginsPath = "plugins-path"
)

var (
	configPath  = "config.yml"
	pluginsPath = "plugins"
)

func envString(name string, value string) string {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	return envString
}

func initEnv() {
	configPath = envString(envConfigPath, configPath)
	pluginsPath = envString(envPluginsPath, pluginsPath)
}

func initFlags() {
	flag.StringVar(&configPath, clfConfigPath, configPath, "path of the config file")
	flag.StringVar(&pluginsPath, clfPluginsPath, pluginsPath, "path to the plugins folder")
	flag.Parse()
}

var logger logr.Logger

func init() {
	initEnv()
	initFlags()
	initConfig()
	initPlugins()

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to init logger; err: %s", err)
	}
	logger = zapr.NewLogger(zapLog)
}

func main() {
	logger.Info("loading proxy",
		"configFile", configPath,
	)

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

	plugins, err := infrared.LoadPluginsFromDir(pluginsPath)
	if err != nil {
		logger.Error(err, "failed to load plugins")
		return
	}

	pluginManager := infrared.PluginManager{
		Proxies: []infrared.Proxy{bedrockProxy, javaProxy},
		Plugins: plugins,
		Log:     logger,
	}

	if err := pluginManager.EnablePlugins(); err != nil {
		logger.Error(err, "failed to enable plugins")
		return
	}

	logger.Info("starting proxy")

	go bedrockProxy.Start(logger)
	go javaProxy.Start(logger)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	pluginManager.DisablePlugins()
}
