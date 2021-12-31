package bedrock

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/webhook"
	"github.com/sandertv/go-raknet"
	"github.com/spf13/viper"
)

type Config struct{}

type pingStatusConfig struct {
	Edition         string `mapstructure:"edition"`
	ProtocolVersion int    `mapstructure:"protocol_version,omitempty"`
	VersionName     string `mapstructure:"version_name,omitempty"`
	PlayerCount     int    `mapstructure:"player_count,omitempty"`
	MaxPlayerCount  int    `mapstructure:"max_player_count,omitempty"`
	GameMode        string `mapstructure:"game_mode"`
	GameModeNumeric int    `mapstructure:"game_mode_numeric"`
	MOTD            string `mapstructure:"motd,omitempty"`
}

func newPingStatus(cfg pingStatusConfig) PingStatus {
	return PingStatus{
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

type listenerConfig struct {
	Bind                 string           `mapstructure:"bind"`
	PingStatus           pingStatusConfig `mapstructure:"ping_status"`
	ReceiveProxyProtocol bool             `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP        bool             `mapstructure:"receive_real_ip"`
}

func newListener(cfg listenerConfig) Listener {
	return Listener{
		Bind:                 cfg.Bind,
		PingStatus:           newPingStatus(cfg.PingStatus),
		ReceiveProxyProtocol: cfg.ReceiveProxyProtocol,
		ReceiveRealIP:        cfg.ReceiveRealIP,
	}
}

func loadListeners(gatewayID string) ([]Listener, error) {
	key := fmt.Sprintf("bedrock.gateways.%s.listeners", gatewayID)
	ll, ok := viper.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("gateway %q is missing listeners", gatewayID)
	}

	listeners := make([]Listener, len(ll))
	for n := range ll {
		vpr := viper.Sub("defaults.bedrock.gateway.listener")
		lKey := fmt.Sprintf("%s.%d", key, n)
		vMap := viper.GetStringMap(lKey)
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg listenerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		listeners[n] = newListener(cfg)
	}
	return listeners, nil
}

type gatewayConfig struct {
	ClientTimeout         time.Duration `mapstructure:"client_timeout"`
	Servers               []string      `mapstructure:"servers"`
	ServerNotFoundMessage string        `mapstructure:"server_not_found_message"`
}

func newGateway(id string, cfg gatewayConfig) (infrared.Gateway, error) {
	listeners, err := loadListeners(id)
	if err != nil {
		return nil, err
	}

	return &Gateway{
		ID:                    id,
		Listeners:             listeners,
		ClientTimeout:         cfg.ClientTimeout,
		ServerIDs:             cfg.Servers,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
	}, nil
}

func (cfg Config) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("bedrock.gateways") {
		vpr := viper.Sub("defaults.bedrock.gateway")
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg gatewayConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		gateway, err := newGateway(id, cfg)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

type serverConfig struct {
	Domains            []string      `mapstructure:"domains"`
	Address            string        `mapstructure:"address"`
	ProxyBind          string        `mapstructure:"proxy_bind"`
	DialTimeout        time.Duration `mapstructure:"dial_timeout"`
	SendProxyProtocol  bool          `mapstructure:"send_proxy_protocol"`
	DialTimeoutMessage string        `mapstructure:"dial_timeout_message"`
}

func newServer(id string, cfg serverConfig) infrared.Server {
	return &Server{
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

func (cfg Config) LoadServers() ([]infrared.Server, error) {
	var servers []infrared.Server
	for id, v := range viper.GetStringMap("bedrock.servers") {
		vpr := viper.Sub("defaults.bedrock.server")
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg serverConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		servers = append(servers, newServer(id, cfg))
	}

	return servers, nil
}

type cpnConfig struct {
	Count int `mapstructure:"count"`
}

func (cfg Config) LoadCPNs() ([]infrared.CPN, error) {
	var cpnCfg cpnConfig
	if err := viper.UnmarshalKey("processing_nodes", &cpnCfg); err != nil {
		return nil, err
	}

	cpns := make([]infrared.CPN, cpnCfg.Count)
	for n := range cpns {
		cpns[n].ConnProcessor = ConnProcessor{}
	}

	return cpns, nil
}

type webhookConfig struct {
	ClientTimeout time.Duration `mapstructure:"client_timeout"`
	URL           string        `mapstructure:"url"`
	Events        []string      `mapstructure:"events"`
}

func newWebhook(id string, cfg webhookConfig) webhook.Webhook {
	return webhook.Webhook{
		ID: id,
		HTTPClient: &http.Client{
			Timeout: cfg.ClientTimeout,
		},
		URL:        cfg.URL,
		EventTypes: cfg.Events,
	}
}

func (cfg Config) LoadWebhooks() ([]webhook.Webhook, error) {
	var webhooks []webhook.Webhook
	for id, v := range viper.GetStringMap("webhooks") {
		vpr := viper.Sub("defaults.webhook")
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg webhookConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, newWebhook(id, cfg))
	}

	return webhooks, nil
}
