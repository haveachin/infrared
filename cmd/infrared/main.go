package main

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/haveachin/infrared"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	envPrefix     = "INFRARED_"
	envDebug      = envPrefix + "DEBUG"
	envColor      = envPrefix + "COLOR"
	envConfigPath = envPrefix + "CONFIG_PATH"
)

const (
	clfDebug      = "debug"
	clfColor      = "color"
	clfConfigPath = "config-path"
)

var (
	debug      = false
	color      = true
	configPath = "./configs"
)

func envBool(name string, value bool) bool {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	envBool, err := strconv.ParseBool(envString)
	if err != nil {
		return value
	}

	return envBool
}

func envString(name string, value string) string {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	return envString
}

func initEnv() {
	debug = envBool(envDebug, debug)
	color = envBool(envColor, color)
	configPath = envString(envConfigPath, configPath)
}

func initFlags() {
	flag.BoolVar(&debug, clfDebug, debug, "starts infrared in debug mode")
	flag.BoolVar(&color, clfColor, color, "enables color in console logs")
	flag.StringVar(&configPath, clfConfigPath, configPath, "path of all proxy configs")
	flag.Parse()
}

func init() {
	initEnv()
	initFlags()

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    !color,
	})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

func main() {
	vprs, err := infrared.ReadAllProxyConfigs(configPath)
	if err != nil {
		log.Info().Err(err)
		return
	}

	gateway := infrared.NewGateway()

	for _, vpr := range vprs {
		if _, err := gateway.AddProxyByViper(vpr); err != nil {
			log.Err(err)
		}
	}

	if err := gateway.ListenAndServe(); err != nil {
		log.Err(err)
	}
}
