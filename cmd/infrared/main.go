package main

import (
	"os"

	"github.com/haveachin/infrared"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	envPrefix     = "infrared"
	envAddress    = "address"
	envConfigPath = "config_path"
)

func init() {
	viper.SetEnvPrefix(envPrefix)
	viper.BindEnv(envAddress)
	viper.BindEnv(envConfigPath)

	pflag.String(envAddress, ":25565", "address that the proxy listens to")
	pflag.String(envConfigPath, "./configs/", "path of all your server configs")

	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {
	address := viper.Get(envAddress).(string)
	if address == "" {
		address = viper.GetString(envAddress)
	}

	configPath := viper.Get(envConfigPath).(string)
	if configPath == "" {
		configPath = viper.GetString(envConfigPath)
	}

	vprs, err := infrared.ReadAllConfigs(configPath)
	if err != nil {
		log.Info().Err(err)
		return
	}

	gateway := infrared.NewGateway(vprs)
	gateway.Open()
}
