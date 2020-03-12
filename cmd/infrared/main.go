package main

import (
	"log"
	"os"

	"github.com/haveachin/infrared"
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
		log.Fatal(err)
		return
	}

	info := log.New(os.Stdout, "[INFO]: ", log.Ldate|log.Ltime)
	warning := log.New(os.Stdout, "[WRNG]: ", log.Ldate|log.Ltime)
	critical := log.New(os.Stdout, "[CRTL]: ", log.Ldate|log.Ltime)

	gateway := infrared.NewGateway(vprs, info, warning, critical)
	gateway.Open()
}
