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
	if err := viper.UnmarshalKey("java.processingNodes", &cpnCfg); err != nil {
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
	if err := viper.UnmarshalKey("java.chanCaps", &chanCapsCfg); err != nil {
		return infrared.ProxyChanCaps{}, err
	}

	return newJavaChanCaps(chanCapsCfg), nil
}

type javaServerConfig struct {
	Domains            []string
	Address            string
	ProxyBind          string
	SendProxyProtocol  bool
	SendRealIP         bool
	DialTimeout        time.Duration
	DialTimeoutMessage string
	OverrideStatus     javaOverrideServerStatusConfig
	DialTimeoutStatus  javaDialTimeoutServerStatusConfig
	Webhooks           []string
}

type javaOverrideServerStatusConfig struct {
	VersionName    *string
	ProtocolNumber *int
	MaxPlayerCount *int
	PlayerCount    *int
	PlayerSample   []javaServerStatusPlayerSampleConfig
	IconPath       *string
	MOTD           *string
}

type javaDialTimeoutServerStatusConfig struct {
	VersionName    string
	ProtocolNumber int
	MaxPlayerCount int
	PlayerCount    int
	PlayerSample   []javaServerStatusPlayerSampleConfig
	IconPath       string
	MOTD           string
}

type javaServerStatusPlayerSampleConfig struct {
	Name string
	UUID string
}

type javaListenerConfig struct {
	Bind                  string
	ReceiveProxyProtocol  bool
	ReceiveRealIP         bool
	ServerNotFoundMessage string
	ServerNotFoundStatus  javaDialTimeoutServerStatusConfig
}

type javaGatewayConfig struct {
	Binds                 []string
	ReceiveProxyProtocol  bool
	ReceiveRealIP         bool
	ClientTimeout         time.Duration
	Servers               []string
	ServerNotFoundMessage string
}

type javaCpnConfig struct {
	Count         int
	ClientTimeout time.Duration
}

type javaChanCapsConfig struct {
	CPN      int
	Server   int
	ConnPool int
}

func newJavaListener(cfg javaListenerConfig) java.Listener {
	return java.Listener{
		Bind:                  cfg.Bind,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
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
