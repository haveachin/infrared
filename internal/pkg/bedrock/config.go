package bedrock

import (
	"fmt"
	"github.com/haveachin/infrared/pkg/event"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/sandertv/go-raknet"
	"github.com/spf13/viper"
)

type ProxyConfig struct {
	Viper *viper.Viper
}

func (cfg ProxyConfig) ListenerBuilder() infrared.ListenerBuilder {
	return func(addr string) (net.Listener, error) {
		return raknet.Listen(addr)
	}
}

func (cfg ProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, data := range cfg.Viper.GetStringMap("bedrock.gateways") {
		defaults := cfg.Viper.Sub("defaults.bedrock.gateway").AllSettings()
		vpr := viper.New()
		if err := vpr.MergeConfigMap(defaults); err != nil {
			return nil, err
		}
		if err := vpr.MergeConfigMap(data.(map[string]interface{})); err != nil {
			return nil, err
		}
		var c gatewayConfig
		if err := vpr.Unmarshal(&c); err != nil {
			return nil, err
		}
		gateway, err := newGateway(cfg.Viper, id, c)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

func (cfg ProxyConfig) LoadServers() ([]infrared.Server, error) {
	var servers []infrared.Server
	for id, data := range cfg.Viper.GetStringMap("bedrock.servers") {
		defaults := cfg.Viper.Sub("defaults.bedrock.server").AllSettings()
		vpr := viper.New()
		if err := vpr.MergeConfigMap(defaults); err != nil {
			return nil, err
		}
		if err := vpr.MergeConfigMap(data.(map[string]interface{})); err != nil {
			return nil, err
		}
		var cfg serverConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		srv := newServer(id, cfg)
		event.Push(infrared.ServerRegisterEvent{
			Server: srv,
		}, infrared.ServerRegisterEventTopic)
		servers = append(servers, srv)
	}

	return servers, nil
}

func (cfg ProxyConfig) LoadConnProcessor() (infrared.ConnProcessor, error) {
	var cpnCfg connProcessorConfig
	if err := cfg.Viper.UnmarshalKey("bedrock.processingNode", &cpnCfg); err != nil {
		return nil, err
	}
	return &InfraredConnProcessor{
		ConnProcessor: ConnProcessor{
			ClientTimeout: cpnCfg.ClientTimeout,
		},
	}, nil
}

func (cfg ProxyConfig) LoadProxySettings() (infrared.ProxySettings, error) {
	var chanCapsCfg chanCapsConfig
	if err := cfg.Viper.UnmarshalKey("bedrock.chanCap", &chanCapsCfg); err != nil {
		return infrared.ProxySettings{}, err
	}
	cpnCount := cfg.Viper.GetInt("bedrock.processingNode.count")

	return newChanCaps(chanCapsCfg, cpnCount), nil
}

type pingStatusConfig struct {
	Edition         string
	ProtocolVersion int
	VersionName     string
	PlayerCount     int
	MaxPlayerCount  int
	GameMode        string
	GameModeNumeric int
	MOTD            string
}

type listenerConfig struct {
	Bind                  string
	PingStatus            pingStatusConfig
	ReceiveProxyProtocol  bool
	ServerNotFoundMessage string
}

type gatewayConfig struct {
}

type serverConfig struct {
	Domains            []string
	Address            string
	ProxyBind          string
	DialTimeout        time.Duration
	SendProxyProtocol  bool
	DialTimeoutMessage string
	Gateways           []string
	Webhooks           []string
}

type connProcessorConfig struct {
	Count         int
	ClientTimeout time.Duration
}

type chanCapsConfig struct {
	ConnProcessor int
	Server        int
	ConnPool      int
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

func newListener(id string, cfg listenerConfig) Listener {
	return Listener{
		ID:                    id,
		Bind:                  cfg.Bind,
		PingStatus:            newPingStatus(cfg.PingStatus),
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
	}
}

func newGateway(v *viper.Viper, id string, cfg gatewayConfig) (infrared.Gateway, error) {
	listeners, err := loadListeners(v, id)
	if err != nil {
		return nil, err
	}

	return &InfraredGateway{
		gateway: Gateway{
			ID:        id,
			Listeners: listeners,
		},
	}, nil
}

func newServer(id string, cfg serverConfig) infrared.Server {
	return &InfraredServer{
		Server: Server{
			ID:      id,
			Domains: cfg.Domains,
			Dialer: raknet.Dialer{
				UpstreamDialer: &net.Dialer{
					Timeout: cfg.DialTimeout,
					LocalAddr: &net.UDPAddr{
						IP: net.ParseIP(cfg.ProxyBind),
					},
				},
			},
			Address:            cfg.Address,
			SendProxyProtocol:  cfg.SendProxyProtocol,
			DialTimeoutMessage: cfg.DialTimeoutMessage,
			GatewayIDs:         cfg.Gateways,
			WebhookIDs:         cfg.Webhooks,
		},
	}
}

func newChanCaps(cfg chanCapsConfig, cpnCount int) infrared.ProxySettings {
	return infrared.ProxySettings{
		CPNCount: cpnCount,
		ChannelCaps: infrared.ProxyChannelCaps{
			ConnProcessor: cfg.ConnProcessor,
			Server:        cfg.Server,
			ConnPool:      cfg.ConnPool,
		},
	}
}

func loadListeners(v *viper.Viper, gatewayID string) ([]Listener, error) {
	key := fmt.Sprintf("bedrock.gateways.%s.listeners", gatewayID)
	var listeners []Listener
	for id, data := range v.GetStringMap(key) {
		defaults := v.Sub("defaults.bedrock.gateway.listener").AllSettings()
		vpr := viper.New()
		if err := vpr.MergeConfigMap(defaults); err != nil {
			return nil, err
		}
		if err := vpr.MergeConfigMap(data.(map[string]interface{})); err != nil {
			return nil, err
		}
		var cfg listenerConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		listeners = append(listeners, newListener(id, cfg))
	}
	return listeners, nil
}
