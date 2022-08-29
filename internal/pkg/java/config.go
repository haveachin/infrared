package java

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/imdario/mergo"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
)

type ServerConfig struct {
	Domains            []string                      `mapstructure:"domains" json:"domains,omitempty"`
	Address            string                        `mapstructure:"address" json:"address,omitempty"`
	ProxyBind          string                        `mapstructure:"proxyBind" json:"proxyBind,omitempty"`
	SendProxyProtocol  bool                          `mapstructure:"sendProxyProtocol" json:"sendProxyProtocol,omitempty"`
	SendRealIP         bool                          `mapstructure:"sendRealIP" json:"sendRealIP,omitempty"`
	OverrideAddress    bool                          `mapstructure:"overrideAddress" json:"overrideAddress,omitempty"`
	DialTimeout        time.Duration                 `mapstructure:"dialTimeout" json:"dialTimeout,omitempty"`
	DialTimeoutMessage string                        `mapstructure:"dialTimeoutMessage" json:"dialTimeoutMessage,omitempty"`
	OverrideStatus     OverrideServerStatusConfig    `mapstructure:"overrideStatus" json:"overrideStatus,omitempty"`
	DialTimeoutStatus  DialTimeoutServerStatusConfig `mapstructure:"dialTimeoutStatus" json:"dialTimeoutStatus,omitempty"`
	Gateways           []string                      `mapstructure:"gateways" json:"gateways,omitempty"`
}

type OverrideServerStatusConfig struct {
	VersionName    *string                          `mapstructure:"versionName" json:"versionName,omitempty"`
	ProtocolNumber *int                             `mapstructure:"protocolNumber" json:"protocolNumber,omitempty"`
	MaxPlayerCount *int                             `mapstructure:"maxPlayerCount" json:"maxPlayerCount,omitempty"`
	PlayerCount    *int                             `mapstructure:"playerCount" json:"playerCount,omitempty"`
	PlayerSample   []ServerStatusPlayerSampleConfig `mapstructure:"playerSample" json:"playerSample,omitempty"`
	IconPath       *string                          `mapstructure:"iconPath" json:"iconPath,omitempty"`
	MOTD           *string                          `mapstructure:"motd" json:"motd,omitempty"`
}

type DialTimeoutServerStatusConfig struct {
	VersionName    string                           `mapstructure:"versionName" json:"motd,omitempty"`
	ProtocolNumber int                              `mapstructure:"protocolNumber" json:"protocolNumber,omitempty"`
	MaxPlayerCount int                              `mapstructure:"maxPlayerCount" json:"maxPlayerCount,omitempty"`
	PlayerCount    int                              `mapstructure:"playerCount" json:"playerCount,omitempty"`
	PlayerSample   []ServerStatusPlayerSampleConfig `mapstructure:"playerSample" json:"playerSample,omitempty"`
	IconPath       string                           `mapstructure:"iconPath" json:"iconPath,omitempty"`
	MOTD           string                           `mapstructure:"motd" json:"motd,omitempty"`
}

type ServerStatusPlayerSampleConfig struct {
	Name string `mapstructure:"name,omitempty"`
	UUID string `mapstructure:"uuid,omitempty"`
}

type ListenerConfig struct {
	Bind                  string                        `mapstructure:"bind" json:"bind,omitempty"`
	ReceiveProxyProtocol  bool                          `mapstructure:"receiveProxyProtocol" json:"receiveProxyProtocol,omitempty"`
	ReceiveRealIP         bool                          `mapstructure:"receiveRealIP,omitempty" json:"receiveRealIP,omitempty"`
	ServerNotFoundMessage string                        `mapstructure:"serverNotFoundMessage,omitempty" json:"serverNotFoundMessage,omitempty"`
	ServerNotFoundStatus  DialTimeoutServerStatusConfig `mapstructure:"serverNotFoundStatus,omitempty" json:"serverNotFoundStatus,omitempty"`
}

type GatewayConfig struct {
	Listeners             map[string]ListenerConfig `mapstructure:"listeners" json:"listeners,omitempty"`
	ServerNotFoundMessage string                    `mapstructure:"serverNotFoundMessage,omitempty" json:"serverNotFoundMessage,omitempty"`
}

type ConnProcessorConfig struct {
	Count         int           `mapstructure:"count" json:"count,omitempty"`
	ClientTimeout time.Duration `mapstructure:"clientTimeout" json:"clientTimeout,omitempty"`
}

type ChanCapsConfig struct {
	ConnProcessor int `mapstructure:"connProcessor" json:"connProcessor,omitempty"`
	Server        int `mapstructure:"server" json:"server,omitempty"`
	ConnPool      int `mapstructure:"connPool" json:"connPool,omitempty"`
}

type ProxyConfig struct {
	Gateways      map[string]GatewayConfig `mapstructure:"gateways" json:"gateways,omitempty"`
	Servers       map[string]ServerConfig  `mapstructure:"servers" json:"servers,omitempty"`
	ChanCaps      ChanCapsConfig           `mapstructure:"chanCaps" json:"chanCaps,omitempty"`
	ConnProcessor ConnProcessorConfig      `mapstructure:"processingNode" json:"processingNode,omitempty"`
}

type ProxyConfigDefaults struct {
	Gateway struct {
		Listener              ListenerConfig `mapstructure:"listener,omitempty" json:"listener,omitempty"`
		ServerNotFoundMessage string         `mapstructure:"serverNotFoundMessage,omitempty" json:"serverNotFoundMessage,omitempty"`
	} `mapstructure:"gateway,omitempty" json:"gateway,omitempty"`
	Server ServerConfig `mapstructure:"server,omitempty" json:"server,omitempty"`
}

type Config struct {
	Java     ProxyConfig `mapstructure:"java" json:"java,omitempty"`
	Defaults struct {
		Java ProxyConfigDefaults `mapstructure:"java" json:"java,omitempty"`
	} `mapstructure:"defaults,omitempty" json:"defaults,omitempty"`
}

func NewProxyConfigFromMap(cfg map[string]interface{}) (infrared.ProxyConfig, error) {
	var javaCfg Config
	if err := config.Unmarshal(cfg, &javaCfg); err != nil {
		return nil, err
	}

	return &javaCfg, nil
}

func (cfg Config) ListenerBuilder() infrared.ListenerBuilder {
	return func(addr string) (net.Listener, error) {
		return net.Listen("tcp", addr)
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
	return &InfraredConnProcessor{
		ConnProcessor: ConnProcessor{
			ClientTimeout: cfg.Java.ConnProcessor.ClientTimeout,
		},
	}, nil
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
	status, err := newDialTimeoutServerStatus(cfg.ServerNotFoundStatus)
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
	overrideStatus, err := newOverrideServerStatus(cfg.OverrideStatus)
	if err != nil {
		return nil, err
	}

	dialTimeoutStatus, err := newDialTimeoutServerStatus(cfg.DialTimeoutStatus)
	if err != nil {
		return nil, err
	}

	respJSON := dialTimeoutStatus.ResponseJSON()
	bb, err := json.Marshal(respJSON)
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

	return &InfraredServer{
		Server: Server{
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
			OverrideAddress:       cfg.OverrideAddress,
			DialTimeoutMessage:    cfg.DialTimeoutMessage,
			OverrideStatus:        overrideStatus,
			DialTimeoutStatusJSON: string(bb),
			GatewayIDs:            cfg.Gateways,
			Host:                  host,
			Port:                  port,
		},
	}, nil
}

func newOverrideServerStatus(cfg OverrideServerStatusConfig) (OverrideStatusResponse, error) {
	var icon string
	if cfg.IconPath != nil {
		var err error
		icon, err = loadImageAndEncodeToBase64String(*cfg.IconPath)
		if err != nil {
			return OverrideStatusResponse{}, err
		}
	}

	return OverrideStatusResponse{
		VersionName:    cfg.VersionName,
		ProtocolNumber: cfg.ProtocolNumber,
		MaxPlayerCount: cfg.MaxPlayerCount,
		PlayerCount:    cfg.PlayerCount,
		Icon:           &icon,
		MOTD:           cfg.MOTD,
		PlayerSamples:  newServerStatusPlayerSample(cfg.PlayerSample),
	}, nil
}

func newDialTimeoutServerStatus(cfg DialTimeoutServerStatusConfig) (DialTimeoutStatusResponse, error) {
	icon, err := loadImageAndEncodeToBase64String(cfg.IconPath)
	if err != nil {
		return DialTimeoutStatusResponse{}, err
	}
	return DialTimeoutStatusResponse{
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
		playerSamples[n] = PlayerSample{
			Name: cfg.Name,
			UUID: cfg.UUID,
		}
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

func loadImageAndEncodeToBase64String(path string) (string, error) {
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
