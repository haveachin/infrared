package java

import (
	"net"
	"net/http"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/webhook"
	"github.com/spf13/viper"
)

type Config struct{}

type gatewayConfig struct {
	Binds                 []string      `mapstructure:"binds"`
	ReceiveProxyProtocol  bool          `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP         bool          `mapstructure:"receive_real_ip"`
	ClientTimeout         time.Duration `mapstructure:"client_timeout"`
	Servers               []string      `mapstructure:"servers"`
	ServerNotFoundMessage string        `mapstructure:"server_not_found_message"`
}

func newGateway(id string, cfg gatewayConfig) infrared.Gateway {
	return &Gateway{
		ID:                    id,
		Binds:                 cfg.Binds,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
		ClientTimeout:         cfg.ClientTimeout,
		ServerIDs:             cfg.Servers,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
	}
}

func (cfg Config) LoadGateways() ([]infrared.Gateway, error) {
	vpr := viper.Sub("defaults.java.gateway")

	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("java.gateways") {
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg gatewayConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		gateways = append(gateways, newGateway(id, cfg))
	}

	return gateways, nil
}

type serverConfig struct {
	Domains           []string                  `mapstructure:"domains"`
	Address           string                    `mapstructure:"address"`
	ProxyBind         string                    `mapstructure:"proxy_bind"`
	DialTimeout       time.Duration             `mapstructure:"dial_timeout"`
	SendProxyProtocol bool                      `mapstructure:"send_proxy_protocol"`
	SendRealIP        bool                      `mapstructure:"send_real_ip"`
	DisconnectMessage string                    `mapstructure:"disconnect_message"`
	OnlineStatus      onlineServerStatusConfig  `mapstructure:"online_status"`
	OfflineStatus     offlineServerStatusConfig `mapstructure:"offline_status"`
}

type onlineServerStatusConfig struct {
	VersionName    *string                          `mapstructure:"version_name,omitempty"`
	ProtocolNumber *int                             `mapstructure:"protocol_number,omitempty"`
	MaxPlayer      *int                             `mapstructure:"max_players,omitempty"`
	PlayersOnline  *int                             `mapstructure:"players_online,omitempty"`
	PlayerSample   []serverStatusPlayerSampleConfig `mapstructure:"player_sample,omitempty"`
	IconPath       *string                          `mapstructure:"icon_path,omitempty"`
	MOTD           *string                          `mapstructure:"motd,omitempty"`
}

type offlineServerStatusConfig struct {
	VersionName    string                           `mapstructure:"version_name"`
	ProtocolNumber int                              `mapstructure:"protocol_number"`
	MaxPlayer      int                              `mapstructure:"max_players"`
	PlayersOnline  int                              `mapstructure:"players_online"`
	PlayerSample   []serverStatusPlayerSampleConfig `mapstructure:"player_sample"`
	IconPath       string                           `mapstructure:"icon_path"`
	MOTD           string                           `mapstructure:"motd"`
}

type serverStatusPlayerSampleConfig struct {
	Name string `mapstructure:"name"`
	UUID string `mapstructure:"uuid"`
}

func newServer(id string, cfg serverConfig) infrared.Server {
	return &Server{
		ID:      id,
		Domains: cfg.Domains,
		Dialer: net.Dialer{
			Timeout: cfg.DialTimeout,
			LocalAddr: &net.TCPAddr{
				IP: net.ParseIP(cfg.ProxyBind),
			},
		},
		Address:           cfg.Address,
		SendProxyProtocol: cfg.SendProxyProtocol,
		SendRealIP:        cfg.SendRealIP,
		DisconnectMessage: cfg.DisconnectMessage,
		OnlineStatus:      newOnlineServerStatus(cfg.OnlineStatus),
		OfflineStatus:     newOfflineServerStatus(cfg.OfflineStatus),
	}
}

func newOnlineServerStatus(cfg onlineServerStatusConfig) OnlineStatusResponse {
	return OnlineStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayers:     cfg.MaxPlayer,
		PlayersOnline:  cfg.PlayersOnline,
		IconPath:       cfg.IconPath,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}
}

func newOfflineServerStatus(cfg offlineServerStatusConfig) OfflineStatusResponse {
	return OfflineStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayers:     cfg.MaxPlayer,
		PlayersOnline:  cfg.PlayersOnline,
		IconPath:       cfg.IconPath,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}
}

func newServerStatusPlayerSample(cfgs []serverStatusPlayerSampleConfig) []PlayerSample {
	playerSamples := make([]PlayerSample, len(cfgs))
	for n, cfg := range cfgs {
		playerSamples[n] = PlayerSample{
			Name: cfg.Name,
			UUID: cfg.UUID,
		}
	}
	return playerSamples
}

func (cfg Config) LoadServers() ([]infrared.Server, error) {
	vpr := viper.Sub("defaults.java.server")

	var servers []infrared.Server
	for id, v := range viper.GetStringMap("java.servers") {
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
	vpr := viper.Sub("defaults.java.webhook")

	var webhooks []webhook.Webhook
	for id, v := range viper.GetStringMap("java.webhooks") {
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
