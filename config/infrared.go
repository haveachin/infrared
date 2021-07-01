package config

import (
	"encoding/json"

	"github.com/haveachin/infrared"
)

type ServerConfig struct {
	// MainDomain will be treated as primary key
	MainDomain   string   `json:"mainDomain"`
	ExtraDomains []string `json:"extraDomains"`

	ListenTo          string `json:"listenTo"`
	ProxyBind         string `json:"proxyBind"`
	ProxyTo           string `json:"proxyTo"`
	SendProxyProtocol bool   `json:"sendProxyProtocol"`
	RealIP            bool   `json:"realIp"`

	DialTimeout       int    `json:"dialTimeout"`
	DisconnectMessage string `json:"disconnectMessage"`

	//Need different statusconfig struct
	OnlineStatus  infrared.StatusConfig `json:"onlineStatus"`
	OfflineStatus infrared.StatusConfig `json:"offlineStatus"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MainDomain:        "localhost",
		ListenTo:          ":25565",
		DialTimeout:       1000,
		DisconnectMessage: "Sorry {{username}}, but the server is offline.",
		OfflineStatus: infrared.StatusConfig{
			VersionName:    "Infrared 1.17",
			ProtocolNumber: 755,
			MaxPlayers:     20,
			MOTD:           "Powered by Infrared",
		},
	}
}

func (cfg *ServerConfig) UpdateServerConfig(newCfg ServerConfig) error {
	var defaultCfg map[string]interface{}
	bb, err := json.Marshal(DefaultServerConfig())
	err = json.Unmarshal(bb, &defaultCfg)

	var loadedCfg map[string]interface{}
	bb, err = json.Marshal(newCfg)
	err = json.Unmarshal(bb, &loadedCfg)

	for k, v := range loadedCfg {
		defaultCfg[k] = v
	}

	bb, err = json.Marshal(defaultCfg)
	err = json.Unmarshal(bb, cfg)
	return err
}

func jsonToServerCfg(bb []byte) (ServerConfig, error) {
	cfg := ServerConfig{}
	if err := json.Unmarshal(bb, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
