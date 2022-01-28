package main

import (
	"fmt"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/sandertv/go-raknet"
	"github.com/spf13/viper"
)

type BedrockProxyConfig struct{}

func (cfg BedrockProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("bedrock.gateways") {
		vpr := viper.Sub("defaults.bedrock.gateway")
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

func (cfg BedrockProxyConfig) LoadCPNs() ([]infrared.CPN, error) {
	var cpnCfg bedrockCPNConfig
	if err := viper.UnmarshalKey("bedrock.processing_nodes", &cpnCfg); err != nil {
		return nil, err
	}

	cpns := make([]infrared.CPN, cpnCfg.Count)
	for n := range cpns {
		cpns[n].ConnProcessor = bedrock.ConnProcessor{}
	}

	return cpns, nil
}

func (cfg BedrockProxyConfig) LoadWebhooks() ([]webhook.Webhook, error) {
	vpr := viper.Sub("defaults.bedrock.webhook")

	var webhooks []webhook.Webhook
	for id, v := range viper.GetStringMap("bedrock.webhooks") {
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg javaWebhookConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, newJavaWebhook(id, cfg))
	}

	return webhooks, nil
}

type bedrockPingStatusConfig struct {
	Edition         string `mapstructure:"edition"`
	ProtocolVersion int    `mapstructure:"protocol_version,omitempty"`
	VersionName     string `mapstructure:"version_name,omitempty"`
	PlayerCount     int    `mapstructure:"player_count,omitempty"`
	MaxPlayerCount  int    `mapstructure:"max_player_count,omitempty"`
	GameMode        string `mapstructure:"game_mode"`
	GameModeNumeric int    `mapstructure:"game_mode_numeric"`
	MOTD            string `mapstructure:"motd,omitempty"`
}

type bedrockListenerConfig struct {
	Bind                 string                  `mapstructure:"bind"`
	PingStatus           bedrockPingStatusConfig `mapstructure:"ping_status"`
	ReceiveProxyProtocol bool                    `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP        bool                    `mapstructure:"receive_real_ip"`
}

type bedrockGatewayConfig struct {
	ClientTimeout         time.Duration `mapstructure:"client_timeout"`
	Servers               []string      `mapstructure:"servers"`
	ServerNotFoundMessage string        `mapstructure:"server_not_found_message"`
}

type bedrockServerConfig struct {
	Domains            []string      `mapstructure:"domains"`
	Address            string        `mapstructure:"address"`
	ProxyBind          string        `mapstructure:"proxy_bind"`
	DialTimeout        time.Duration `mapstructure:"dial_timeout"`
	SendProxyProtocol  bool          `mapstructure:"send_proxy_protocol"`
	DialTimeoutMessage string        `mapstructure:"dial_timeout_message"`
}

type bedrockCPNConfig struct {
	Count int `mapstructure:"count"`
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
		Bind:                 cfg.Bind,
		PingStatus:           newBedrockPingStatus(cfg.PingStatus),
		ReceiveProxyProtocol: cfg.ReceiveProxyProtocol,
		ReceiveRealIP:        cfg.ReceiveRealIP,
	}
}

func newBedrockGateway(id string, cfg bedrockGatewayConfig) (*bedrock.Gateway, error) {
	listeners, err := loadBedrockListeners(id)
	if err != nil {
		return nil, err
	}

	return &bedrock.Gateway{
		ID:                    id,
		Listeners:             listeners,
		ClientTimeout:         cfg.ClientTimeout,
		ServerIDs:             cfg.Servers,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
	}, nil
}

func newBedrockServer(id string, cfg bedrockServerConfig) *bedrock.Server {
	return &bedrock.Server{
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
