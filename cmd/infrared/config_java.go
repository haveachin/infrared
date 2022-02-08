package main

import (
	"fmt"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/spf13/viper"
)

type JavaProxyConfig struct{}

func (cfg JavaProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("java.gateways") {
		vpr := viper.Sub("defaults.java.gateway")
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg javaGatewayConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		gateway, err := newJavaGateway(id, cfg)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

func (cfg JavaProxyConfig) LoadServers() ([]infrared.Server, error) {
	var servers []infrared.Server
	for id, v := range viper.GetStringMap("java.servers") {
		vpr := viper.Sub("defaults.java.server")
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg javaServerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		servers = append(servers, newJavaServer(id, cfg))
	}

	return servers, nil
}

func (cfg JavaProxyConfig) LoadCPNs() ([]infrared.CPN, error) {
	var cpnCfg javaCpnConfig
	if err := viper.UnmarshalKey("java.processing_nodes", &cpnCfg); err != nil {
		return nil, err
	}

	cpns := make([]infrared.CPN, cpnCfg.Count)
	for n := range cpns {
		cpns[n].ConnProcessor = java.ConnProcessor{
			ClientTimeout: cpnCfg.ClientTimeout,
		}
	}

	return cpns, nil
}

func (cfg JavaProxyConfig) LoadChanCaps() (infrared.ProxyChanCaps, error) {
	var chanCapsCfg javaChanCapsConfig
	if err := viper.UnmarshalKey("java.chan_caps", &chanCapsCfg); err != nil {
		return infrared.ProxyChanCaps{}, err
	}

	return newJavaChanCaps(chanCapsCfg), nil
}

type javaServerConfig struct {
	Domains            []string                          `mapstructure:"domains"`
	Address            string                            `mapstructure:"address"`
	ProxyBind          string                            `mapstructure:"proxy_bind"`
	SendProxyProtocol  bool                              `mapstructure:"send_proxy_protocol"`
	SendRealIP         bool                              `mapstructure:"send_real_ip"`
	DialTimeout        time.Duration                     `mapstructure:"dial_timeout"`
	DialTimeoutMessage string                            `mapstructure:"dial_timeout_message"`
	OverrideStatus     javaOverrideServerStatusConfig    `mapstructure:"override_status"`
	DialTimeoutStatus  javaDialTimeoutServerStatusConfig `mapstructure:"dial_timeout_status"`
	Webhooks           []string                          `mapstructure:"webhooks"`
}

type javaOverrideServerStatusConfig struct {
	VersionName    *string                              `mapstructure:"version_name,omitempty"`
	ProtocolNumber *int                                 `mapstructure:"protocol_number,omitempty"`
	MaxPlayerCount *int                                 `mapstructure:"max_player_count,omitempty"`
	PlayerCount    *int                                 `mapstructure:"player_count,omitempty"`
	PlayerSample   []javaServerStatusPlayerSampleConfig `mapstructure:"player_sample,omitempty"`
	IconPath       *string                              `mapstructure:"icon_path,omitempty"`
	MOTD           *string                              `mapstructure:"motd,omitempty"`
}

type javaDialTimeoutServerStatusConfig struct {
	VersionName    string                               `mapstructure:"version_name"`
	ProtocolNumber int                                  `mapstructure:"protocol_number"`
	MaxPlayerCount int                                  `mapstructure:"max_player_count"`
	PlayerCount    int                                  `mapstructure:"player_count"`
	PlayerSample   []javaServerStatusPlayerSampleConfig `mapstructure:"player_sample"`
	IconPath       string                               `mapstructure:"icon_path"`
	MOTD           string                               `mapstructure:"motd"`
}

type javaServerStatusPlayerSampleConfig struct {
	Name string `mapstructure:"name"`
	UUID string `mapstructure:"uuid"`
}

type javaListenerConfig struct {
	Bind                  string                            `mapstructure:"bind"`
	ReceiveProxyProtocol  bool                              `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP         bool                              `mapstructure:"receive_real_ip"`
	ClientTimeout         time.Duration                     `mapstructure:"client_timeout"`
	ServerNotFoundMessage string                            `mapstructure:"server_not_found_message"`
	ServerNotFoundStatus  javaDialTimeoutServerStatusConfig `mapstructure:"server_not_found_status"`
}

type javaGatewayConfig struct {
	Binds                 []string      `mapstructure:"bind"`
	ReceiveProxyProtocol  bool          `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP         bool          `mapstructure:"receive_real_ip"`
	ClientTimeout         time.Duration `mapstructure:"client_timeout"`
	Servers               []string      `mapstructure:"servers"`
	ServerNotFoundMessage string        `mapstructure:"server_not_found_message"`
}

type javaCpnConfig struct {
	Count         int           `mapstructure:"count"`
	ClientTimeout time.Duration `mapstructure:"client_timeout"`
}

type javaChanCapsConfig struct {
	CPN      int `mapstructure:"cpn"`
	Server   int `mapstructure:"server"`
	ConnPool int `mapstructure:"conn_pool"`
}

func newJavaListener(cfg javaListenerConfig) java.Listener {
	return java.Listener{
		Bind:                  cfg.Bind,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
		ClientTimeout:         cfg.ClientTimeout,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
		ServerNotFoundStatus:  newJavaDialTimeoutServerStatus(cfg.ServerNotFoundStatus),
	}
}

func newJavaGateway(id string, cfg javaGatewayConfig) (infrared.Gateway, error) {
	listeners, err := loadJavaListeners(id)
	if err != nil {
		return nil, err
	}

	return &java.InfraredGateway{
		Gateway: java.Gateway{
			ID:        id,
			Listeners: listeners,
			ServerIDs: cfg.Servers,
		},
	}, nil
}

func newJavaServer(id string, cfg javaServerConfig) infrared.Server {
	return &java.InfraredServer{
		Server: java.Server{
			ID:      id,
			Domains: cfg.Domains,
			Dialer: net.Dialer{
				Timeout: cfg.DialTimeout,
				LocalAddr: &net.TCPAddr{
					IP: net.ParseIP(cfg.ProxyBind),
				},
			},
			Address:            cfg.Address,
			SendProxyProtocol:  cfg.SendProxyProtocol,
			SendRealIP:         cfg.SendRealIP,
			DialTimeoutMessage: cfg.DialTimeoutMessage,
			OverrideStatus:     newJavaOverrideServerStatus(cfg.OverrideStatus),
			DialTimeoutStatus:  newJavaDialTimeoutServerStatus(cfg.DialTimeoutStatus),
			WebhookIDs:         cfg.Webhooks,
		},
	}
}

func newJavaOverrideServerStatus(cfg javaOverrideServerStatusConfig) java.OverrideStatusResponse {
	return java.OverrideStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		IconPath:       cfg.IconPath,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newJavaServerStatusPlayerSample(cfg.PlayerSample),
	}
}

func newJavaDialTimeoutServerStatus(cfg javaDialTimeoutServerStatusConfig) java.DialTimeoutStatusResponse {
	return java.DialTimeoutStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		IconPath:       cfg.IconPath,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newJavaServerStatusPlayerSample(cfg.PlayerSample),
	}
}

func newJavaServerStatusPlayerSample(cfgs []javaServerStatusPlayerSampleConfig) []java.PlayerSample {
	playerSamples := make([]java.PlayerSample, len(cfgs))
	for n, cfg := range cfgs {
		playerSamples[n] = java.PlayerSample{
			Name: cfg.Name,
			UUID: cfg.UUID,
		}
	}
	return playerSamples
}

func newJavaChanCaps(cfg javaChanCapsConfig) infrared.ProxyChanCaps {
	return infrared.ProxyChanCaps{
		CPN:      cfg.CPN,
		Server:   cfg.Server,
		ConnPool: cfg.ConnPool,
	}
}

func loadJavaListeners(gatewayID string) ([]java.Listener, error) {
	key := fmt.Sprintf("java.gateways.%s.listeners", gatewayID)
	ll, ok := viper.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("gateway %q is missing listeners", gatewayID)
	}

	listeners := make([]java.Listener, len(ll))
	for n := range ll {
		vpr := viper.Sub("defaults.java.gateway.listener")
		if vpr == nil {
			vpr = viper.New()
		}
		lKey := fmt.Sprintf("%s.%d", key, n)
		vMap := viper.GetStringMap(lKey)
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg javaListenerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		listeners[n] = newJavaListener(cfg)
	}
	return listeners, nil
}
