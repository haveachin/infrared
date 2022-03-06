package main

import (
	"fmt"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/sandertv/go-raknet"
	"github.com/spf13/viper"
)

type BedrockProxyConfig struct{}

func (cfg BedrockProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("bedrock.gateways") {
		vpr := viper.Sub("defaults.bedrock.gateway")
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg bedrockGatewayConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		gateway, err := newBedrockGateway(id, cfg)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

func (cfg BedrockProxyConfig) LoadServers() ([]infrared.Server, error) {
	var servers []infrared.Server
	for id, v := range viper.GetStringMap("bedrock.servers") {
		vpr := viper.Sub("defaults.bedrock.server")
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg bedrockServerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		servers = append(servers, newBedrockServer(id, cfg))
	}

	return servers, nil
}

func (cfg BedrockProxyConfig) LoadConnProcessor() (infrared.ConnProcessor, error) {
	var cpnCfg bedrockCPNConfig
	if err := viper.UnmarshalKey("bedrock.processingNodes", &cpnCfg); err != nil {
		return nil, err
	}
	return &bedrock.InfraredConnProcessor{
		ConnProcessor: bedrock.ConnProcessor{
			ClientTimeout: cpnCfg.ClientTimeout,
		},
	}, nil
}

func (cfg BedrockProxyConfig) LoadProxySettings() (infrared.ProxySettings, error) {
	var chanCapsCfg bedrockChanCapsConfig
	if err := viper.UnmarshalKey("bedrock.chanCap", &chanCapsCfg); err != nil {
		return infrared.ProxySettings{}, err
	}
	cpnCount := viper.GetInt("bedrock.processingNode.count")

	return newBedrockChanCaps(chanCapsCfg, cpnCount), nil
}

type bedrockPingStatusConfig struct {
	Edition         string
	ProtocolVersion int
	VersionName     string
	PlayerCount     int
	MaxPlayerCount  int
	GameMode        string
	GameModeNumeric int
	MOTD            string
}

type bedrockListenerConfig struct {
	Bind                  string
	PingStatus            bedrockPingStatusConfig
	ReceiveProxyProtocol  bool
	ServerNotFoundMessage string
}

type bedrockGatewayConfig struct {
	Servers []string
}

type bedrockServerConfig struct {
	Domains            []string
	Address            string
	ProxyBind          string
	DialTimeout        time.Duration
	SendProxyProtocol  bool
	DialTimeoutMessage string
	Webhooks           []string
}

type bedrockCPNConfig struct {
	Count         int
	ClientTimeout time.Duration
}

type bedrockChanCapsConfig struct {
	ConnProcessor int
	Server        int
	ConnPool      int
}

func newBedrockPingStatus(cfg bedrockPingStatusConfig) bedrock.PingStatus {
	return bedrock.PingStatus{
		Edition:         cfg.Edition,
		ProtocolVersion: cfg.ProtocolVersion,
		VersionName:     cfg.VersionName,
		PlayerCount:     cfg.PlayerCount,
		MaxPlayerCount:  cfg.MaxPlayerCount,
		GameMode:        cfg.GameMode,
		GameModeNumeric: cfg.GameModeNumeric,
		MOTD:            cfg.MOTD,
	}
}

func newBedrockListener(cfg bedrockListenerConfig) bedrock.Listener {
	return bedrock.Listener{
		Bind:                  cfg.Bind,
		PingStatus:            newBedrockPingStatus(cfg.PingStatus),
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
	}
}

func newBedrockGateway(id string, cfg bedrockGatewayConfig) (infrared.Gateway, error) {
	listeners, err := loadBedrockListeners(id)
	if err != nil {
		return nil, err
	}

	return &bedrock.InfraredGateway{
		Gateway: bedrock.Gateway{
			ID:        id,
			Listeners: listeners,
			ServerIDs: cfg.Servers,
		},
	}, nil
}

func newBedrockServer(id string, cfg bedrockServerConfig) infrared.Server {
	return &bedrock.InfraredServer{
		Server: bedrock.Server{
			ID:      id,
			Domains: cfg.Domains,
			Dialer: raknet.Dialer{
				UpstreamDialer: &net.Dialer{
					LocalAddr: &net.UDPAddr{
						IP: net.ParseIP(cfg.ProxyBind),
					},
				},
			},
			DialTimeout:        cfg.DialTimeout,
			Address:            cfg.Address,
			SendProxyProtocol:  cfg.SendProxyProtocol,
			DialTimeoutMessage: cfg.DialTimeoutMessage,
			WebhookIDs:         cfg.Webhooks,
		},
	}
}

func newBedrockChanCaps(cfg bedrockChanCapsConfig, cpnCount int) infrared.ProxySettings {
	return infrared.ProxySettings{
		CPNCount: cpnCount,
		ChannelCaps: infrared.ProxyChannelCaps{
			ConnProcessor: cfg.ConnProcessor,
			Server:        cfg.Server,
			ConnPool:      cfg.ConnPool,
		},
	}
}

func loadBedrockListeners(gatewayID string) ([]bedrock.Listener, error) {
	key := fmt.Sprintf("bedrock.gateways.%s.listeners", gatewayID)
	ll, ok := viper.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("gateway %q is missing listeners", gatewayID)
	}

	listeners := make([]bedrock.Listener, len(ll))
	for n := range ll {
		vpr := viper.Sub("defaults.bedrock.gateway.listener")
		if vpr == nil {
			vpr = viper.New()
		}
		lKey := fmt.Sprintf("%s.%d", key, n)
		vMap := viper.GetStringMap(lKey)
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg bedrockListenerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		listeners[n] = newBedrockListener(cfg)
	}
	return listeners, nil
}
