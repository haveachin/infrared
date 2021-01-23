package main

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var ErrFileIsDir = errors.New("file is a dir")

// config is a data representation of a proxy configuration
type config struct {
	vpr           *viper.Viper
	DomainName    string
	ListenTo      string
	ProxyTo       string
	ProxyProtocol bool
	Timeout       string
}

// loadAndWatchConfig loads config from a file path and then starts watching
// it for changes. On change the config will automatically reload itself
func loadAndWatchConfig(path string) (config, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return config{}, err
	}

	if stat.IsDir() {
		return config{}, ErrFileIsDir
	}

	fileName := stat.Name()
	extension := filepath.Ext(fileName)
	configName := fileName[0 : len(fileName)-len(extension)]

	vpr := viper.New()
	vpr.AddConfigPath(path)
	vpr.SetConfigName(configName)
	vpr.SetConfigType(strings.TrimPrefix(extension, "."))

	cfg := config{vpr: vpr}
	cfg.setDefaults()
	if err := cfg.reload(); err != nil {
		return config{}, err
	}

	vpr.OnConfigChange(func(in fsnotify.Event) {
		if in.Op != fsnotify.Write {
			return
		}
		_ = cfg.reload()
	})
	vpr.WatchConfig()

	return cfg, err
}

// reload loads the config from it's source file
func (cfg *config) reload() error {
	if err := cfg.vpr.ReadInConfig(); err != nil {
		return err
	}

	if err := cfg.vpr.Unmarshal(cfg); err != nil {
		return err
	}

	return nil
}

func (cfg *config) setDefaults() {
	cfg.vpr.SetDefault("ListenTo", ":25565")
	cfg.vpr.SetDefault("Timeout", "5m")

	cfg.vpr.SetDefault("Process.DNSServer", "127.0.0.11")
	cfg.vpr.SetDefault("Server.DisconnectMessage", "Hey §e$username§r! The server was sleeping but it is starting now.")
	cfg.vpr.SetDefault("Server.Version", "Infrared 1.15.2")
	cfg.vpr.SetDefault("Server.Protocol", 578)
	cfg.vpr.SetDefault("Server.Motd", "Powered by Infrared")
	cfg.vpr.SetDefault("Server.MaxPlayers", 20)
	cfg.vpr.SetDefault("Server.PlayersOnline", 0)
}
