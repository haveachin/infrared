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

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type DockerOldConfig struct {
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

type PlayerSampleOldConfig struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type StatusOldConfig struct {
	VersionName    string                  `json:"versionName"`
	ProtocolNumber int                     `json:"protocolNumber"`
	MaxPlayers     int                     `json:"maxPlayers"`
	PlayersOnline  int                     `json:"playersOnline"`
	PlayerSamples  []PlayerSampleOldConfig `json:"playerSamples"`
	IconPath       string                  `json:"iconPath"`
	MOTD           string                  `json:"motd"`
}

type CallbackServerOldConfig struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type ProxyOldConfig struct {
	DomainName        string                  `json:"domainName"`
	ListenTo          string                  `json:"listenTo"`
	ProxyTo           string                  `json:"proxyTo"`
	ProxyBind         string                  `json:"proxyBind"`
	SpoofForcedHost   string                  `json:"spoofForcedHost"`
	ProxyProtocol     bool                    `json:"proxyProtocol"`
	RealIP            bool                    `json:"realIp"`
	Timeout           int                     `json:"timeout"`
	DisconnectMessage string                  `json:"disconnectMessage"`
	Docker            DockerOldConfig         `json:"docker"`
	OnlineStatus      StatusOldConfig         `json:"onlineStatus"`
	OfflineStatus     StatusOldConfig         `json:"offlineStatus"`
	CallbackServer    CallbackServerOldConfig `json:"callbackServer"`
}

type ServerNewConfig struct {
	Domains            []string               `mapstructure:"domains"`
	Address            *string                `mapstructure:"address"`
	ProxyBind          *string                `mapstructure:"proxyBind"`
	SendProxyProtocol  *bool                  `mapstructure:"sendProxyProtocol"`
	SendRealIP         *bool                  `mapstructure:"sendRealIP"`
	OverrideAddress    *bool                  `mapstructure:"overrideAddress"`
	DialTimeout        *time.Duration         `mapstructure:"dialTimeout"`
	DialTimeoutMessage *string                `mapstructure:"dialTimeoutMessage"`
	OverrideStatus     *ServerStatusNewConfig `mapstructure:"overrideStatus"`
	DialTimeoutStatus  *ServerStatusNewConfig `mapstructure:"dialTimeoutStatus"`
	Gateways           []string               `mapstructure:"gateways"`
}

type ServerStatusNewConfig struct {
	VersionName    *string                             `mapstructure:"versionName"`
	ProtocolNumber *int                                `mapstructure:"protocolNumber"`
	MaxPlayerCount *int                                `mapstructure:"maxPlayerCount"`
	PlayerCount    *int                                `mapstructure:"playerCount"`
	PlayerSample   []ServerStatusPlayerSampleNewConfig `mapstructure:"playerSample"`
	IconPath       *string                             `mapstructure:"iconPath"`
	MOTD           *string                             `mapstructure:"motd"`
}

type ServerStatusPlayerSampleNewConfig struct {
	Name *string `mapstructure:"name,omitempty"`
	UUID *string `mapstructure:"uuid,omitempty"`
}

type ListenerNewConfig struct {
	Bind                  *string                `mapstructure:"bind"`
	ReceiveProxyProtocol  *bool                  `mapstructure:"receiveProxyProtocol"`
	ReceiveRealIP         *bool                  `mapstructure:"receiveRealIP,omitempty"`
	ServerNotFoundMessage *string                `mapstructure:"serverNotFoundMessage,omitempty"`
	ServerNotFoundStatus  *ServerStatusNewConfig `mapstructure:"serverNotFoundStatus,omitempty"`
}

type GatewayNewConfig struct {
	Listeners             map[string]ListenerNewConfig `mapstructure:"listeners"`
	ServerNotFoundMessage *string                      `mapstructure:"serverNotFoundMessage,omitempty"`
}

type ProxyNewConfig struct {
	Gateways map[string]GatewayNewConfig `mapstructure:"gateways"`
	Servers  map[string]ServerNewConfig  `mapstructure:"servers"`
}

type GatewayProxyNewConfigDefaults struct {
	Listener              *ListenerNewConfig `mapstructure:"listener,omitempty"`
	ServerNotFoundMessage *string            `mapstructure:"serverNotFoundMessage,omitempty"`
}

type ProxyNewConfigDefaults struct {
	Gateway *GatewayProxyNewConfigDefaults `mapstructure:"gateway,omitempty"`
	Server  *ServerNewConfig               `mapstructure:"server,omitempty"`
}

type NewConfigDefaults struct {
	Java *ProxyNewConfigDefaults `mapstructure:"java"`
}

type NewConfig struct {
	Java     *ProxyNewConfig    `mapstructure:"java"`
	Defaults *NewConfigDefaults `mapstructure:"defaults,omitempty"`
}

var (
	migrateMerged    bool
	migrateDryRun    bool
	migratePrintCfgs bool
	migrateVerbose   bool

	migrateCmd = &cobra.Command{
		Use:   "migrate [old config dir] [new config dir]",
		Short: "Migrates old config files from v1 to the new v2 format.",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("two arguments are expected")
			}
			oldCfgPath := args[0]
			migratedCfgPath := args[1]

			if err := os.Chdir(workingDir); err != nil {
				return err
			}

			if _, err := os.Stat(oldCfgPath); err != nil {
				return err
			}

			if _, err := os.Stat(migratedCfgPath); err != nil {
				return err
			}

			oldCfgs, err := readOldConfigs(args[0])
			if err != nil {
				return err
			}

			if migrateVerbose {
				fmt.Printf("%d old configs read from %s\n", len(oldCfgs), oldCfgPath)
			}

			cfgs, err := convertOldConfigsToNew(oldCfgs)
			if err != nil {
				return err
			}

			if migrateVerbose {
				fmt.Printf("%d new configs would be written to %s\n", len(cfgs), migratedCfgPath)
			}

			if migratePrintCfgs {
				for name, cfg := range cfgs {
					bb, err := json.MarshalIndent(cfg, "", "  ")
					if err != nil {
						return err
					}

					fmt.Printf("%s\n%s\n", name, string(bb))
				}
			}

			if migrateDryRun {
				return nil
			}

			return writeNewConfigs(args[1], cfgs)
		},
	}
)

func init() {
	migrateCmd.Flags().BoolVarP(&migrateMerged, "merged", "m", false, "if the configs should be merged into one file")
	migrateCmd.Flags().BoolVarP(&migrateDryRun, "dry-run", "d", false, "runs the migration without changing or writing any files")
	migrateCmd.Flags().BoolVarP(&migratePrintCfgs, "print-migrated-configs", "p", false, "prints all the migrated config files")
	migrateCmd.Flags().BoolVarP(&migrateVerbose, "verbose", "v", false, "prints additional info while migrating")
}

func readOldConfigs(dir string) (map[string]ProxyOldConfig, error) {
	cfgs := map[string]ProxyOldConfig{}
	readConfig := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info == nil || info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		cfg := ProxyOldConfig{}
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

func convertOldConfigsToNew(oldCfgs map[string]ProxyOldConfig) (map[string]NewConfig, error) {
	cfgs := map[string]NewConfig{}
	// TODO: Fix migration of old configs
	/*listenToCfg := NewConfig{
		Java: &ProxyNewConfig{
			Gateways: map[string]GatewayNewConfig{
				"default": {
					Listeners: map[string]ListenerNewConfig{},
				},
			},
		},
	}

	for name, oldCfg := range oldCfgs {
		listenerCfgs := listenToCfg.Java.Gateways["default"].Listeners
		listenTo := oldCfg.ListenTo
		if listenTo == "" {
			listenTo = ":25565"
		}
		addr, err := net.ResolveTCPAddr("tcp", listenTo)
		if err != nil {
			return nil, err
		}
		listenerExists := false
		for _, lCfg := range listenerCfgs {
			if lCfg.Bind == addr.String() {
				listenerExists = true
				break
			}
		}

		if !listenerExists {
			listenerCfgs[name] = ListenerNewConfig{
				Bind: addr.String(),
			}
		}

		overridePlayerSamples := make([]ServerStatusPlayerSampleNewConfig, len(oldCfg.OnlineStatus.PlayerSamples))
		for i, playerSample := range oldCfg.OnlineStatus.PlayerSamples {
			overridePlayerSamples[i] = ServerStatusPlayerSampleNewConfig{
				Name: playerSample.Name,
				UUID: playerSample.UUID,
			}
		}

		dialTimeoutPlayerSamples := make([]ServerStatusPlayerSampleNewConfig, len(oldCfg.OnlineStatus.PlayerSamples))
		for i, playerSample := range oldCfg.OnlineStatus.PlayerSamples {
			dialTimeoutPlayerSamples[i] = ServerStatusPlayerSampleNewConfig{
				Name: playerSample.Name,
				UUID: playerSample.UUID,
			}
		}

		cfgs[name] = NewConfig{
			Java: ProxyNewConfig{
				Servers: map[string]ServerNewConfig{
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
						OverrideStatus: ServerStatusNewConfig{
							VersionName:    &oldCfg.OnlineStatus.VersionName,
							ProtocolNumber: &oldCfg.OnlineStatus.ProtocolNumber,
							MaxPlayerCount: &oldCfg.OnlineStatus.MaxPlayers,
							PlayerCount:    &oldCfg.OnlineStatus.PlayersOnline,
							IconPath:       &oldCfg.OnlineStatus.IconPath,
							PlayerSample:   overridePlayerSamples,
							MOTD:           &oldCfg.OnlineStatus.MOTD,
						},
						DialTimeoutStatus: DialTimeoutServerStatusNewConfig{
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
	cfgs["default"] = listenToCfg*/
	return cfgs, nil
}

func writeNewConfigs(dir string, cfgs map[string]NewConfig) error {
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
