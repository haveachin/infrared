package main

import (
	"encoding/json"
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
		srv, err := newJavaServer(id, cfg)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}

	return servers, nil
}

func (cfg JavaProxyConfig) LoadConnProcessor() (infrared.ConnProcessor, error) {
	var cpnCfg javaConnProcessorConfig
	if err := viper.UnmarshalKey("java.processingNode", &cpnCfg); err != nil {
		return nil, err
	}

	return &java.InfraredConnProcessor{
		ConnProcessor: java.ConnProcessor{
			ClientTimeout: cpnCfg.ClientTimeout,
		},
	}, nil
}

func (cfg JavaProxyConfig) LoadProxySettings() (infrared.ProxySettings, error) {
	var chanCapsCfg javaChanCapsConfig
	if err := viper.UnmarshalKey("java.chanCap", &chanCapsCfg); err != nil {
		return infrared.ProxySettings{}, err
	}
	cpnCount := viper.GetInt("java.processingNode.count")

	return newJavaChanCaps(chanCapsCfg, cpnCount), nil
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

type javaConnProcessorConfig struct {
	Count         int
	ClientTimeout time.Duration
}

type javaChanCapsConfig struct {
	ConnProcessor int
	Server        int
	ConnPool      int
}

func newJavaListener(cfg javaListenerConfig) (java.Listener, error) {
	status, err := newJavaDialTimeoutServerStatus(cfg.ServerNotFoundStatus)
	if err != nil {
		return java.Listener{}, err
	}

	return java.Listener{
		Bind:                  cfg.Bind,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
		ServerNotFoundStatus:  status,
	}, nil
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

func newJavaServer(id string, cfg javaServerConfig) (infrared.Server, error) {
	overrideStatus, err := newJavaOverrideServerStatus(cfg.OverrideStatus)
	if err != nil {
		return nil, err
	}

	dialTimeoutStatus, err := newJavaDialTimeoutServerStatus(cfg.DialTimeoutStatus)
	if err != nil {
		return nil, err
	}

	respJSON := dialTimeoutStatus.ResponseJSON()
	bb, err := json.Marshal(respJSON)
	if err != nil {
		return nil, err
	}

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
			Addr:                  cfg.Address,
			SendProxyProtocol:     cfg.SendProxyProtocol,
			SendRealIP:            cfg.SendRealIP,
			DialTimeoutMessage:    cfg.DialTimeoutMessage,
			OverrideStatus:        overrideStatus,
			DialTimeoutStatusJSON: string(bb),
			WebhookIDs:            cfg.Webhooks,
		},
	}, nil
}

func newJavaOverrideServerStatus(cfg javaOverrideServerStatusConfig) (java.OverrideStatusResponse, error) {
	var icon string
	if cfg.IconPath != nil {
		var err error
		icon, err = loadImageAndEncodeToBase64String(*cfg.IconPath)
		if err != nil {
			return java.OverrideStatusResponse{}, err
		}
	}

	return java.OverrideStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		Icon:           &icon,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newJavaServerStatusPlayerSample(cfg.PlayerSample),
	}, nil
}

func newJavaDialTimeoutServerStatus(cfg javaDialTimeoutServerStatusConfig) (java.DialTimeoutStatusResponse, error) {
	icon, err := loadImageAndEncodeToBase64String(cfg.IconPath)
	if err != nil {
		return java.DialTimeoutStatusResponse{}, err
	}
	return java.DialTimeoutStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		Icon:           icon,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newJavaServerStatusPlayerSample(cfg.PlayerSample),
	}, nil
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

func newJavaChanCaps(cfg javaChanCapsConfig, cpnCount int) infrared.ProxySettings {
	return infrared.ProxySettings{
		CPNCount: cpnCount,
		ChannelCaps: infrared.ProxyChannelCaps{
			ConnProcessor: cfg.ConnProcessor,
			Server:        cfg.Server,
			ConnPool:      cfg.ConnPool,
		},
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
		var err error
		listeners[n], err = newJavaListener(cfg)
		if err != nil {
			return nil, err
		}
	}
	return listeners, nil
}
