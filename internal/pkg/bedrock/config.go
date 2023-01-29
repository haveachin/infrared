package bedrock

import (
	"fmt"
	"net"
	"time"

	"github.com/imdario/mergo"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/sandertv/go-raknet"
)

type ServerConfig struct {
	Domains            []string      `mapstructure:"domains"`
	Address            string        `mapstructure:"address"`
	ProxyBind          string        `mapstructure:"proxyBind"`
	SendProxyProtocol  bool          `mapstructure:"sendProxyProtocol"`
	DialTimeout        time.Duration `mapstructure:"dialTimeout"`
	DialTimeoutMessage string        `mapstructure:"dialTimeoutMessage"`
	Gateways           []string      `mapstructure:"gateways"`
}

type PingStatusConfig struct {
	Edition         string `mapstructure:"edition"`
	ProtocolVersion int    `mapstructure:"protocolVersion"`
	VersionName     string `mapstructure:"versionName"`
	PlayerCount     int    `mapstructure:"playerCount"`
	MaxPlayerCount  int    `mapstructure:"maxPlayerCount"`
	GameMode        string `mapstructure:"gameMode"`
	GameModeNumeric int    `mapstructure:"gameModeNumeric"`
	MOTD            string `mapstructure:"motd"`
}

type ListenerConfig struct {
	Bind                  string           `mapstructure:"bind"`
	PingStatus            PingStatusConfig `mapstructure:"pingStatus"`
	ReceiveProxyProtocol  bool             `mapstructure:"receiveProxyProtocol"`
	ServerNotFoundMessage string           `mapstructure:"serverNotFoundMessage"`
	Compression           string           `mapstructure:"compression"`
}

type GatewayConfig struct {
	Listeners             map[string]ListenerConfig `mapstructure:"listeners"`
	ServerNotFoundMessage string                    `mapstructure:"serverNotFoundMessage"`
}

type ConnProcessorConfig struct {
	Count         int           `mapstructure:"count"`
	ClientTimeout time.Duration `mapstructure:"clientTimeout"`
}

type ChanCapsConfig struct {
	ConnProcessor int `mapstructure:"connProcessor"`
	Server        int `mapstructure:"server"`
	ConnPool      int `mapstructure:"connPool"`
}

type MiddlewareSettings struct {
	RateLimiter RateLimiterSettings `mapstructure:"rateLimiter"`
}

type RateLimiterSettings struct {
	Enable       bool          `mapstructure:"enable"`
	RequestLimit int           `mapstructure:"requestLimit"`
	WindowLength time.Duration `mapstructure:"windowLength"`
}

type ProxyConfig struct {
	Gateways      map[string]GatewayConfig `mapstructure:"gateways"`
	Servers       map[string]ServerConfig  `mapstructure:"servers"`
	ChanCaps      ChanCapsConfig           `mapstructure:"chanCaps"`
	ConnProcessor ConnProcessorConfig      `mapstructure:"processingNode"`
	Middlewares   MiddlewareSettings       `mapstructure:"middlewares"`
}

type ProxyConfigDefaults struct {
	Gateway struct {
		Listener              ListenerConfig `mapstructure:"listener"`
		ServerNotFoundMessage string         `mapstructure:"serverNotFoundMessage"`
	} `mapstructure:"gateway"`
	Server ServerConfig `mapstructure:"server"`
}

type Config struct {
	Bedrock  ProxyConfig `mapstructure:"bedrock"`
	Defaults struct {
		Bedrock ProxyConfigDefaults `mapstructure:"bedrock"`
	} `mapstructure:"defaults"`
}

func NewProxyConfigFromMap(cfg map[string]any) (infrared.ProxyConfig, error) {
	var bedrockCfg Config
	if err := config.Unmarshal(cfg, &bedrockCfg); err != nil {
		return nil, err
	}

	return &bedrockCfg, nil
}

func (cfg Config) ListenerBuilder() infrared.ListenerBuilder {
	return func(addr string) (net.Listener, error) {
		pc := &proxyProtocolPacketConn{}
		lc := raknet.ListenConfig{
			UpstreamPacketListener: pc,
		}

		rl, err := lc.Listen(addr)
		if err != nil {
			return nil, err
		}

		return &proxyProtocolListener{
			Listener:                rl,
			proxyProtocolPacketConn: pc,
		}, nil
	}
}

func (cfg Config) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, gwCfg := range cfg.Bedrock.Gateways {
		c := GatewayConfig{
			ServerNotFoundMessage: cfg.Defaults.Bedrock.Gateway.ServerNotFoundMessage,
		}

		if err := mergo.Merge(&c, gwCfg); err != nil {
			return nil, err
		}

		lCfgs, err := cfg.loadListeners(id)
		if err != nil {
			return nil, err
		}
		c.Listeners = lCfgs

		gateway, err := newGateway(id, c)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

func (cfg Config) LoadServers() ([]infrared.Server, error) {
	var servers []infrared.Server
	for id, srvCfg := range cfg.Bedrock.Servers {
		var c ServerConfig
		if err := mergo.Merge(&c, cfg.Defaults.Bedrock.Server); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&c, srvCfg); err != nil {
			return nil, err
		}

		servers = append(servers, newServer(id, c))
	}
	return servers, nil
}

func (cfg Config) LoadConnProcessor() (infrared.ConnProcessor, error) {
	return &InfraredConnProcessor{
		ConnProcessor: ConnProcessor{
			ClientTimeout: cfg.Bedrock.ConnProcessor.ClientTimeout,
		},
	}, nil
}

func (cfg Config) LoadProxySettings() (infrared.ProxySettings, error) {
	cpnCount := cfg.Bedrock.ConnProcessor.Count
	return newChanCaps(cfg.Bedrock.ChanCaps, cpnCount), nil
}

func (cfg Config) LoadMiddlewareSettings() infrared.MiddlewareSettings {
	return infrared.MiddlewareSettings{
		RateLimiter: infrared.RateLimiterSettings{
			Enable:       cfg.Bedrock.Middlewares.RateLimiter.Enable,
			RequestLimit: cfg.Bedrock.Middlewares.RateLimiter.RequestLimit,
			WindowLength: cfg.Bedrock.Middlewares.RateLimiter.WindowLength,
		},
	}
}

func (cfg Config) loadListeners(gatewayID string) (map[string]ListenerConfig, error) {
	listenerCfgs := map[string]ListenerConfig{}
	for id, lCfg := range cfg.Bedrock.Gateways[gatewayID].Listeners {
		var c ListenerConfig
		if err := mergo.Merge(&c, cfg.Defaults.Bedrock.Gateway.Listener); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&c, lCfg); err != nil {
			return nil, err
		}
		listenerCfgs[id] = c
	}
	return listenerCfgs, nil
}

func newPingStatus(cfg PingStatusConfig) PingStatus {
	return PingStatus(cfg)
}

func newListener(id string, cfg ListenerConfig) (Listener, error) {
	compression, ok := packet.CompressionByName(cfg.Compression)
	if !ok {
		return Listener{}, fmt.Errorf("compression with name %q is not supported", cfg.Compression)
	}

	return Listener{
		ID:                    id,
		Bind:                  cfg.Bind,
		PingStatus:            newPingStatus(cfg.PingStatus),
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
		Compression:           compression,
	}, nil
}

func newGateway(id string, cfg GatewayConfig) (infrared.Gateway, error) {
	var listeners []Listener
	for _, lCfg := range cfg.Listeners {
		l, err := newListener(id, lCfg)
		if err != nil {
			return nil, err
		}

		listeners = append(listeners, l)
	}

	return &InfraredGateway{
		gateway: Gateway{
			ID:        id,
			Listeners: listeners,
		},
	}, nil
}

func newServer(id string, cfg ServerConfig) infrared.Server {
	return &Server{
		id:      id,
		domains: cfg.Domains,
		dialer: raknet.Dialer{
			UpstreamDialer: &net.Dialer{
				Timeout: cfg.DialTimeout,
				LocalAddr: &net.UDPAddr{
					IP: net.ParseIP(cfg.ProxyBind),
				},
			},
		},
		address:                 cfg.Address,
		sendProxyProtocol:       cfg.SendProxyProtocol,
		dialTimeoutDisconnecter: NewPlayerDisconnecter(cfg.DialTimeoutMessage),
		gatewayIDs:              cfg.Gateways,
	}
}

func newChanCaps(cfg ChanCapsConfig, cpnCount int) infrared.ProxySettings {
	return infrared.ProxySettings{
		CPNCount: cpnCount,
		ChannelCaps: infrared.ProxyChannelCaps{
			ConnProcessor: cfg.ConnProcessor,
			Server:        cfg.Server,
			ConnPool:      cfg.ConnPool,
		},
	}
}
