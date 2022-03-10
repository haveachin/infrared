package bedrock

import (
	"fmt"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/sandertv/go-raknet"
	"github.com/spf13/viper"
)

type ProxyConfig struct {
	Viper *viper.Viper
}

func (cfg ProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range cfg.Viper.GetStringMap("bedrock.gateways") {
		vpr := cfg.Viper.Sub("defaults.bedrock.gateway")
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
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
	for id, v := range cfg.Viper.GetStringMap("bedrock.servers") {
		vpr := cfg.Viper.Sub("defaults.bedrock.server")
		if vpr == nil {
			vpr = viper.New()
		}
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

func (cfg ProxyConfig) LoadConnProcessor() (infrared.ConnProcessor, error) {
	var cpnCfg cpnConfig
	if err := cfg.Viper.UnmarshalKey("bedrock.processingNodes", &cpnCfg); err != nil {
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
	Servers []string
}

type serverConfig struct {
	Domains            []string
	Address            string
	ProxyBind          string
	DialTimeout        time.Duration
	SendProxyProtocol  bool
	DialTimeoutMessage string
	Webhooks           []string
}

type cpnConfig struct {
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

func newListener(cfg listenerConfig) Listener {
	return Listener{
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
		Gateway: Gateway{
			ID:        id,
			Listeners: listeners,
			ServerIDs: cfg.Servers,
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
	ll, ok := v.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("gateway %q is missing listeners", gatewayID)
	}

	listeners := make([]Listener, len(ll))
	for n := range ll {
		vpr := v.Sub("defaults.bedrock.gateway.listener")
		if vpr == nil {
			vpr = viper.New()
		}
		lKey := fmt.Sprintf("%s.%d", key, n)
		vMap := v.GetStringMap(lKey)
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
