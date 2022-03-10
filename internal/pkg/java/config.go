package java

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/spf13/viper"
)

type ProxyConfig struct {
	Viper *viper.Viper
}

func (cfg ProxyConfig) LoadGateways() ([]infrared.Gateway, error) {
	var gateways []infrared.Gateway
	for id, v := range cfg.Viper.GetStringMap("java.gateways") {
		vpr := cfg.Viper.Sub("defaults.java.gateway")
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
	for id, v := range cfg.Viper.GetStringMap("java.servers") {
		vpr := cfg.Viper.Sub("defaults.java.server")
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
		srv, err := newServer(id, cfg)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}

	return servers, nil
}

func (cfg ProxyConfig) LoadConnProcessor() (infrared.ConnProcessor, error) {
	var cpnCfg connProcessorConfig
	if err := cfg.Viper.UnmarshalKey("java.processingNode", &cpnCfg); err != nil {
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
	if err := cfg.Viper.UnmarshalKey("java.chanCap", &chanCapsCfg); err != nil {
		return infrared.ProxySettings{}, err
	}
	cpnCount := cfg.Viper.GetInt("java.processingNode.count")

	return newChanCaps(chanCapsCfg, cpnCount), nil
}

type serverConfig struct {
	Domains            []string
	Address            string
	ProxyBind          string
	SendProxyProtocol  bool
	SendRealIP         bool
	DialTimeout        time.Duration
	DialTimeoutMessage string
	OverrideStatus     overrideServerStatusConfig
	DialTimeoutStatus  dialTimeoutServerStatusConfig
	Webhooks           []string
}

type overrideServerStatusConfig struct {
	VersionName    *string
	ProtocolNumber *int
	MaxPlayerCount *int
	PlayerCount    *int
	PlayerSample   []serverStatusPlayerSampleConfig
	IconPath       *string
	MOTD           *string
}

type dialTimeoutServerStatusConfig struct {
	VersionName    string
	ProtocolNumber int
	MaxPlayerCount int
	PlayerCount    int
	PlayerSample   []serverStatusPlayerSampleConfig
	IconPath       string
	MOTD           string
}

type serverStatusPlayerSampleConfig struct {
	Name string
	UUID string
}

type listenerConfig struct {
	Bind                  string
	ReceiveProxyProtocol  bool
	ReceiveRealIP         bool
	ServerNotFoundMessage string
	ServerNotFoundStatus  dialTimeoutServerStatusConfig
}

type gatewayConfig struct {
	Binds                 []string
	ReceiveProxyProtocol  bool
	ReceiveRealIP         bool
	ClientTimeout         time.Duration
	Servers               []string
	ServerNotFoundMessage string
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

func newListener(cfg listenerConfig) (Listener, error) {
	status, err := newDialTimeoutServerStatus(cfg.ServerNotFoundStatus)
	if err != nil {
		return Listener{}, err
	}

	return Listener{
		Bind:                  cfg.Bind,
		ReceiveProxyProtocol:  cfg.ReceiveProxyProtocol,
		ReceiveRealIP:         cfg.ReceiveRealIP,
		ServerNotFoundMessage: cfg.ServerNotFoundMessage,
		ServerNotFoundStatus:  status,
	}, nil
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

func newServer(id string, cfg serverConfig) (infrared.Server, error) {
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
			DialTimeoutMessage:    cfg.DialTimeoutMessage,
			OverrideStatus:        overrideStatus,
			DialTimeoutStatusJSON: string(bb),
			WebhookIDs:            cfg.Webhooks,
		},
	}, nil
}

func newOverrideServerStatus(cfg overrideServerStatusConfig) (OverrideStatusResponse, error) {
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

func newDialTimeoutServerStatus(cfg dialTimeoutServerStatusConfig) (DialTimeoutStatusResponse, error) {
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
	key := fmt.Sprintf("java.gateways.%s.listeners", gatewayID)
	ll, ok := v.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("gateway %q is missing listeners", gatewayID)
	}

	listeners := make([]Listener, len(ll))
	for n := range ll {
		vpr := v.Sub("defaults.java.gateway.listener")
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
		var err error
		listeners[n], err = newListener(cfg)
		if err != nil {
			return nil, err
		}
	}
	return listeners, nil
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
