package infrared

import (
	"github.com/haveachin/infrared/mc/sim"
	"github.com/haveachin/infrared/process"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// ProxyConfig is a data representation of a proxy configuration
type ProxyConfig struct {
	DomainName string
	ListenTo   string
	ProxyTo    string
	Timeout    string
	Process    process.Config
	Server     sim.ServerConfig
}

// ReadAllConfigs reads all files that are in the given path
func ReadAllProxyConfigs(path string) ([]*viper.Viper, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var vprs []*viper.Viper

	for _, file := range files {
		fileName := file.Name()

		log.Info().Msgf("Loading \"%s\"", fileName)

		extension := filepath.Ext(fileName)
		configName := fileName[0 : len(fileName)-len(extension)]

		vpr := viper.New()
		vpr.AddConfigPath(path)
		vpr.SetConfigName(configName)
		vpr.SetConfigType(strings.TrimPrefix(extension, "."))
		loadProxyConfigDefaults(vpr)

		vprs = append(vprs, vpr)
	}

	return vprs, nil
}

// LoadConfig loads the config from the viper configuration
func LoadProxyConfig(vpr *viper.Viper) (ProxyConfig, error) {
	cfg := ProxyConfig{}

	if err := vpr.ReadInConfig(); err != nil {
		return cfg, err
	}

	if err := vpr.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func loadProxyConfigDefaults(vpr *viper.Viper) {
	vpr.SetDefault("ListenTo", ":25565")
	vpr.SetDefault("Timeout", "5m")
	vpr.SetDefault("Process.DNSServer", "127.0.0.11")
	vpr.SetDefault("Server.DisconnectMessage", "Hey §e$username§r! The server was sleeping but it is starting now.")
	vpr.SetDefault("Server.Version", "Infrared 1.15.2")
	vpr.SetDefault("Server.Protocol", 578)
	vpr.SetDefault("Server.Motd", "Powered by Infrared")
	vpr.SetDefault("Server.MaxPlayers", 20)
	vpr.SetDefault("Server.PlayersOnline", 0)
}
