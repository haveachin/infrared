package main

import (
	"bytes"
	_ "embed"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/webhook"
	"github.com/spf13/viper"
)

const configPath = "config.yml"

//go:embed config.yml
var defaultConfig []byte

func init() {
	viper.SetConfigFile(configPath)
	viper.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err := viper.MergeInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

type gatewayConfig struct {
	Binds                []string      `mapstructure:"binds"`
	ReceiveProxyProtocol bool          `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP        bool          `mapstructure:"receive_real_ip"`
	ClientTimeout        time.Duration `mapstructure:"client_timeout"`
	Servers              []string      `mapstructure:"servers"`
}

func newGateway(id string, cfg gatewayConfig) infrared.Gateway {
	return infrared.Gateway{
		ID:                   id,
		Binds:                cfg.Binds,
		ReceiveProxyProtocol: cfg.ReceiveProxyProtocol,
		ReceiveRealIP:        cfg.ReceiveRealIP,
		ClientTimeout:        cfg.ClientTimeout,
		ServerIDs:            cfg.Servers,
	}
}

func loadGateways() ([]infrared.Gateway, error) {
	vpr := viper.Sub("defaults.gateway")

	var gateways []infrared.Gateway
	for id, v := range viper.GetStringMap("gateways") {
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
	Domains           []string           `mapstructure:"domains"`
	Address           string             `mapstructure:"address"`
	ProxyBind         string             `mapstructure:"proxy_bind"`
	DialTimeout       time.Duration      `mapstructure:"dial_timeout"`
	SendProxyProtocol bool               `mapstructure:"send_proxy_protocol"`
	SendRealIP        bool               `mapstructure:"send_real_ip"`
	DisconnectMessage string             `mapstructure:"disconnect_message"`
	OnlineStatus      serverStatusConfig `mapstructure:"online_status"`
	OfflineStatus     serverStatusConfig `mapstructure:"offline_status"`
}

type serverStatusConfig struct {
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
	return infrared.Server{
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
		OnlineStatus:      newServerStatus(cfg.OnlineStatus),
		OfflineStatus:     newServerStatus(cfg.OfflineStatus),
	}
}

func newServerStatus(cfg serverStatusConfig) infrared.StatusResponse {
	return infrared.StatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayers:     cfg.MaxPlayer,
		PlayersOnline:  cfg.PlayersOnline,
		IconPath:       cfg.IconPath,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}
}

func newServerStatusPlayerSample(cfgs []serverStatusPlayerSampleConfig) []infrared.PlayerSample {
	playerSamples := make([]infrared.PlayerSample, len(cfgs))
	for n, cfg := range cfgs {
		playerSamples[n] = infrared.PlayerSample{
			Name: cfg.Name,
			UUID: cfg.UUID,
		}
	}
	return playerSamples
}

func loadServers() ([]infrared.Server, error) {
	vpr := viper.Sub("defaults.server")

	var servers []infrared.Server
	for id, v := range viper.GetStringMap("servers") {
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

func loadCPNs() ([]infrared.CPN, error) {
	var cfg cpnConfig
	if err := viper.UnmarshalKey("connection_processing_nodes", &cfg); err != nil {
		return nil, err
	}

	return make([]infrared.CPN, cfg.Count), nil
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

func loadWebhooks() ([]webhook.Webhook, error) {
	vpr := viper.Sub("defaults.webhook")

	var webhooks []webhook.Webhook
	for id, v := range viper.GetStringMap("webhooks") {
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
