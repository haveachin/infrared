package main

import (
	"log"

	"github.com/spf13/pflag"

	"github.com/spf13/viper"

	"github.com/haveachin/infrared"
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

	proxy := infrared.Proxy{
		Addr:    address,
		Configs: map[string]infrared.Config{},
	}

	configs, err := infrared.ReadConfigs(configPath)
	if err != nil {
		log.Fatal(err)
		return
	}

	for _, config := range configs {
		proxy.Configs[config.DomainName] = config
	}

	log.Fatal(proxy.ListenAndServe())
}
