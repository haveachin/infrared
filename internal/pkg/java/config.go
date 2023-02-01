package java

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/imdario/mergo"
	"golang.org/x/net/proxy"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

type ServerConfig struct {
	Domains            []string                   `mapstructure:"domains"`
	Address            string                     `mapstructure:"address"`
	ProxyBind          string                     `mapstructure:"proxyBind"`
	ProxyPass          string                     `mapstructure:"proxyPass"`
	SendProxyProtocol  bool                       `mapstructure:"sendProxyProtocol"`
	SendRealIP         bool                       `mapstructure:"sendRealIP"`
	OverrideAddress    bool                       `mapstructure:"overrideAddress"`
	DialTimeout        time.Duration              `mapstructure:"dialTimeout" swaggertype:"primitive,string"`
	DialTimeoutMessage string                     `mapstructure:"dialTimeoutMessage"`
	OverrideStatus     OverrideServerStatusConfig `mapstructure:"overrideStatus"`
	DialTimeoutStatus  ServerStatusConfig         `mapstructure:"dialTimeoutStatus"`
	Gateways           []string                   `mapstructure:"gateways"`
	StatusCacheTTL     time.Duration              `mapstructure:"statusCacheTTL" swaggertype:"primitive,string"`
}

type OverrideServerStatusConfig struct {
	VersionName    *string                          `mapstructure:"versionName"`
	ProtocolNumber *int                             `mapstructure:"protocolNumber"`
	MaxPlayerCount *int                             `mapstructure:"maxPlayerCount"`
	PlayerCount    *int                             `mapstructure:"playerCount"`
	PlayerSample   []ServerStatusPlayerSampleConfig `mapstructure:"playerSample"`
	IconPath       *string                          `mapstructure:"iconPath"`
	MOTD           *string                          `mapstructure:"motd"`
}

type ServerStatusConfig struct {
	VersionName    string                           `mapstructure:"versionName"`
	ProtocolNumber int                              `mapstructure:"protocolNumber"`
	MaxPlayerCount int                              `mapstructure:"maxPlayerCount"`
	PlayerCount    int                              `mapstructure:"playerCount"`
	PlayerSample   []ServerStatusPlayerSampleConfig `mapstructure:"playerSample"`
	IconPath       string                           `mapstructure:"iconPath"`
	MOTD           string                           `mapstructure:"motd"`
}

type ServerStatusPlayerSampleConfig struct {
	Name string `mapstructure:"name"`
	UUID string `mapstructure:"uuid"`
}

type ListenerConfig struct {
	Bind                  string             `mapstructure:"bind"`
	ReceiveProxyProtocol  bool               `mapstructure:"receiveProxyProtocol"`
	ReceiveRealIP         bool               `mapstructure:"receiveRealIP"`
	ServerNotFoundMessage string             `mapstructure:"serverNotFoundMessage"`
	ServerNotFoundStatus  ServerStatusConfig `mapstructure:"serverNotFoundStatus"`
}

type GatewayConfig struct {
	Listeners             map[string]ListenerConfig `mapstructure:"listeners"`
	ServerNotFoundMessage string                    `mapstructure:"serverNotFoundMessage"`
}

type ConnProcessorConfig struct {
	Count         int           `mapstructure:"count"`
	ClientTimeout time.Duration `mapstructure:"clientTimeout" swaggertype:"primitive,string"`
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
	WindowLength time.Duration `mapstructure:"windowLength" swaggertype:"primitive,string"`
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
	Java     ProxyConfig `mapstructure:"java"`
	Defaults struct {
		Java ProxyConfigDefaults `mapstructure:"java"`
	} `mapstructure:"defaults"`
}

func NewProxyConfigFromMap(cfg map[string]any) (infrared.ProxyConfig, error) {
	var javaCfg Config
	if err := config.Unmarshal(cfg, &javaCfg); err != nil {
		return nil, err
	}

	return &javaCfg, nil
}

func (cfg Config) ListenerBuilder() infrared.ListenerBuilder {
	return func(addr string) (net.Listener, error) {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}

		return &ProxyProtocolListener{
			Listener: l,
		}, nil
	}
}

func (cfg Config) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, gwCfg := range cfg.Java.Gateways {
		c := GatewayConfig{
			ServerNotFoundMessage: cfg.Defaults.Java.Gateway.ServerNotFoundMessage,
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
	for id, srvCfg := range cfg.Java.Servers {
		var c ServerConfig
		if err := mergo.Merge(&c, cfg.Defaults.Java.Server); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&c, srvCfg); err != nil {
			return nil, err
		}

		server, err := newServer(id, c)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func (cfg Config) LoadConnProcessor() (infrared.ConnProcessor, error) {
	return &ConnProcessor{
		clientTimeout: cfg.Java.ConnProcessor.ClientTimeout,
	}, nil
}

func (cfg Config) LoadMiddlewareSettings() infrared.MiddlewareSettings {
	return infrared.MiddlewareSettings{
		RateLimiter: infrared.RateLimiterSettings{
			Enable:       cfg.Java.Middlewares.RateLimiter.Enable,
			RequestLimit: cfg.Java.Middlewares.RateLimiter.RequestLimit,
			WindowLength: cfg.Java.Middlewares.RateLimiter.WindowLength,
		},
	}
}

func (cfg Config) LoadProxySettings() (infrared.ProxySettings, error) {
	cpnCount := cfg.Java.ConnProcessor.Count
	return newChanCaps(cfg.Java.ChanCaps, cpnCount), nil
}

func (cfg Config) loadListeners(gatewayID string) (map[string]ListenerConfig, error) {
	listenerCfgs := map[string]ListenerConfig{}
	for id, lCfg := range cfg.Java.Gateways[gatewayID].Listeners {
		var c ListenerConfig
		if err := mergo.Merge(&c, cfg.Defaults.Java.Gateway.Listener); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&c, lCfg); err != nil {
			return nil, err
		}
		listenerCfgs[id] = c
	}
	return listenerCfgs, nil
}

func newListener(id string, cfg ListenerConfig) (Listener, error) {
	status, err := NewServerStatus(cfg.ServerNotFoundStatus)
	if err != nil {
		return Listener{}, err
	}

	return Listener{
		ID:                    id,
		Bind:                  cfg.Bind,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
		ServerNotFoundStatus:  status,
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

func newServer(id string, cfg ServerConfig) (infrared.Server, error) {
	overrideStatus, err := NewOverrideServerStatus(cfg.OverrideStatus)
	if err != nil {
		return nil, err
	}

	dialTimeoutStatus, err := NewServerStatus(cfg.DialTimeoutStatus)
	if err != nil {
		return nil, err
	}

	dialTimeoutDisconnector, err := NewPlayerDisconnecter(dialTimeoutStatus.ResponseJSON(), cfg.DialTimeoutMessage)
	if err != nil {
		return nil, err
	}

	host, portString, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	var dialer proxy.Dialer = &net.Dialer{
		Timeout: cfg.DialTimeout,
		LocalAddr: &net.TCPAddr{
			IP: net.ParseIP(cfg.ProxyBind),
		},
	}

	if cfg.ProxyPass != "" {
		proxyURL, err := url.Parse(cfg.ProxyPass)
		if err != nil {
			return nil, err
		}

		dialer, err = proxy.FromURL(proxyURL, dialer)
		if err != nil {
			return nil, err
		}
	}

	srv := &Server{
		id:                      id,
		domains:                 cfg.Domains,
		dialer:                  dialer,
		addr:                    cfg.Address,
		addrHost:                host,
		addrPort:                port,
		sendProxyProtocol:       cfg.SendProxyProtocol,
		sendRealIP:              cfg.SendRealIP,
		overrideAddress:         cfg.OverrideAddress,
		dialTimeoutDisconnector: dialTimeoutDisconnector,
		overrideStatus:          overrideStatus,
		gatewayIDs:              cfg.Gateways,
	}

	srv.statusResponseJSONProvider = &statusResponseJSONProvider{
		server:              srv,
		cacheTTL:            cfg.StatusCacheTTL,
		statusHash:          map[protocol.Version]uint64{},
		statusResponseCache: map[uint64]*statusCacheEntry{},
	}

	return srv, nil
}

func NewOverrideServerStatus(cfg OverrideServerStatusConfig) (OverrideServerStatusResponse, error) {
	var iconPtr *string
	if cfg.IconPath != nil {
		icon, err := LoadImageAndEncodeToBase64String(*cfg.IconPath)
		if err != nil {
			return OverrideServerStatusResponse{}, err
		}
		iconPtr = &icon
	}

	return OverrideServerStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		Icon:           iconPtr,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}, nil
}

func NewServerStatus(cfg ServerStatusConfig) (ServerStatusResponse, error) {
	icon, err := LoadImageAndEncodeToBase64String(cfg.IconPath)
	if err != nil {
		return ServerStatusResponse{}, err
	}

	return ServerStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		Icon:           icon,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}, nil
}

func newServerStatusPlayerSample(cfgs []ServerStatusPlayerSampleConfig) []PlayerSample {
	playerSamples := make([]PlayerSample, len(cfgs))
	for n, cfg := range cfgs {
		playerSamples[n] = PlayerSample(cfg)
	}
	return playerSamples
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

func LoadImageAndEncodeToBase64String(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	imgFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	bb, err := io.ReadAll(imgFile)
	if err != nil {
		return "", err
	}
	img64 := base64.StdEncoding.EncodeToString(bb)

	return fmt.Sprintf("data:image/png;base64,%s", img64), nil
}
