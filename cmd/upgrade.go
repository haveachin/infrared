package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type DockerConfig struct {
	DNSServer     string `json:"dnsServer"`
	ContainerName string `json:"containerName"`
	Timeout       int    `json:"timeout"`
	Portainer     struct {
		Address    string `json:"address"`
		EndpointID string `json:"endpointId"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	} `json:"portainer"`
}

type PlayerSample struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type StatusConfig struct {
	VersionName    string         `json:"versionName"`
	ProtocolNumber int            `json:"protocolNumber"`
	MaxPlayers     int            `json:"maxPlayers"`
	PlayersOnline  int            `json:"playersOnline"`
	PlayerSamples  []PlayerSample `json:"playerSamples"`
	IconPath       string         `json:"iconPath"`
	MOTD           string         `json:"motd"`
}

type CallbackServerConfig struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type OldProxyConfig struct {
	DomainName        string               `json:"domainName"`
	ListenTo          string               `json:"listenTo"`
	ProxyTo           string               `json:"proxyTo"`
	ProxyBind         string               `json:"proxyBind"`
	SpoofForcedHost   string               `json:"spoofForcedHost"`
	ProxyProtocol     bool                 `json:"proxyProtocol"`
	RealIP            bool                 `json:"realIp"`
	Timeout           int                  `json:"timeout"`
	DisconnectMessage string               `json:"disconnectMessage"`
	Docker            DockerConfig         `json:"docker"`
	OnlineStatus      StatusConfig         `json:"onlineStatus"`
	OfflineStatus     StatusConfig         `json:"offlineStatus"`
	CallbackServer    CallbackServerConfig `json:"callbackServer"`
}

var (
	upgradeCmd = &cobra.Command{
		Use:   "upgrade [old config dir] [new config dir]",
		Short: "Upgrades old config files from v1 to the new v2 format.",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("two arguments are expected")
			}

			if err := os.Chdir(workingDir); err != nil {
				return err
			}

			if _, err := os.Stat(args[0]); err != nil {
				return err
			}

			if _, err := os.Stat(args[1]); err != nil {
				return err
			}

			oldCfgs, err := readOldConfigs(args[0])
			if err != nil {
				return err
			}

			cfgs, err := convertOldConfigsToNew(oldCfgs)
			if err != nil {
				return err
			}

			return writeNewConfigs(args[1], cfgs)
		},
	}
)

func readOldConfigs(dir string) (map[string]OldProxyConfig, error) {
	cfgs := map[string]OldProxyConfig{}
	readConfig := func(path string, info fs.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		cfg := OldProxyConfig{}
		if err := json.NewDecoder(f).Decode(&cfg); err != nil {
			return err
		}
		cfgs[info.Name()] = cfg
		return nil
	}

	if err := filepath.Walk(dir, readConfig); err != nil {
		return nil, err
	}

	return cfgs, nil
}

func convertOldConfigsToNew(oldCfgs map[string]OldProxyConfig) (map[string]java.Config, error) {
	listenToCfg := java.Config{
		Java: java.ProxyConfig{
			Gateways: map[string]java.GatewayConfig{
				"default": {
					Listeners: map[string]java.ListenerConfig{},
				},
			},
		},
	}
	cfgs := map[string]java.Config{}
	for name, oldCfg := range oldCfgs {
		listenerCfgs := listenToCfg.Java.Gateways["default"].Listeners
		listenerExists := false
		for _, lCfg := range listenerCfgs {
			if lCfg.Bind == oldCfg.ListenTo {
				listenerExists = true
				break
			}
		}

		if !listenerExists {
			listenerCfgs[name] = java.ListenerConfig{
				Bind: oldCfg.ListenTo,
			}
		}

		overridePlayerSamples := make([]java.ServerStatusPlayerSampleConfig, len(oldCfg.OnlineStatus.PlayerSamples))
		for i, playerSample := range oldCfg.OnlineStatus.PlayerSamples {
			overridePlayerSamples[i] = java.ServerStatusPlayerSampleConfig{
				Name: playerSample.Name,
				UUID: playerSample.UUID,
			}
		}

		dialTimeoutPlayerSamples := make([]java.ServerStatusPlayerSampleConfig, len(oldCfg.OnlineStatus.PlayerSamples))
		for i, playerSample := range oldCfg.OnlineStatus.PlayerSamples {
			dialTimeoutPlayerSamples[i] = java.ServerStatusPlayerSampleConfig{
				Name: playerSample.Name,
				UUID: playerSample.UUID,
			}
		}

		cfgs[name] = java.Config{
			Java: java.ProxyConfig{
				Servers: map[string]java.ServerConfig{
					name: {
						Domains:            []string{oldCfg.DomainName},
						Address:            oldCfg.ProxyTo,
						DialTimeout:        time.Duration(oldCfg.Timeout) * time.Millisecond,
						DialTimeoutMessage: oldCfg.DisconnectMessage,
						ProxyBind:          oldCfg.ProxyBind,
						SendProxyProtocol:  oldCfg.ProxyProtocol,
						SendRealIP:         oldCfg.RealIP,
						Gateways:           []string{"default"},
						OverrideAddress:    oldCfg.SpoofForcedHost != "",
						OverrideStatus: java.OverrideServerStatusConfig{
							VersionName:    &oldCfg.OnlineStatus.VersionName,
							ProtocolNumber: &oldCfg.OnlineStatus.ProtocolNumber,
							MaxPlayerCount: &oldCfg.OnlineStatus.MaxPlayers,
							PlayerCount:    &oldCfg.OnlineStatus.PlayersOnline,
							IconPath:       &oldCfg.OnlineStatus.IconPath,
							PlayerSample:   overridePlayerSamples,
							MOTD:           &oldCfg.OnlineStatus.MOTD,
						},
						DialTimeoutStatus: java.DialTimeoutServerStatusConfig{
							VersionName:    oldCfg.OfflineStatus.VersionName,
							ProtocolNumber: oldCfg.OfflineStatus.ProtocolNumber,
							MaxPlayerCount: oldCfg.OfflineStatus.MaxPlayers,
							PlayerCount:    oldCfg.OfflineStatus.PlayersOnline,
							IconPath:       oldCfg.OfflineStatus.IconPath,
							PlayerSample:   dialTimeoutPlayerSamples,
							MOTD:           oldCfg.OfflineStatus.MOTD,
						},
					},
				},
			},
		}
	}
	cfgs["default"] = listenToCfg
	return cfgs, nil
}

func writeNewConfigs(dir string, cfgs map[string]java.Config) error {
	for name, cfg := range cfgs {
		path := path.Join(dir, fmt.Sprintf("%s.yml", name))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := yaml.NewEncoder(f).Encode(cfg); err != nil {
			return err
		}
	}

	return nil
}
