package proxy

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/server"
)

var (
	playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players",
	}, []string{"host"})
)

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MainDomain:        "localhost",
		ListenTo:          ":25565",
		DialTimeout:       1000,
		DisconnectMessage: "Sorry {{username}}, but the server is offline.",
		OfflineStatus: infrared.StatusConfig{
			VersionName:    "Infrared 1.17",
			ProtocolNumber: 755,
			MaxPlayers:     20,
			MOTD:           "Powered by Infrared",
		},
	}
}

type ServerConfig struct {
	// MainDomain will be treated as primary key
	MainDomain   string   `json:"mainDomain"`
	ExtraDomains []string `json:"extraDomains"`

	ListenTo          string `json:"listenTo"`
	ProxyBind         string `json:"proxyBind"`
	ProxyTo           string `json:"proxyTo"`
	SendProxyProtocol bool   `json:"sendProxyProtocol"`
	RealIP            bool   `json:"realIp"`

	DialTimeout       int    `json:"dialTimeout"`
	DisconnectMessage string `json:"disconnectMessage"`

	//Need different statusconfig struct
	OnlineStatus  infrared.StatusConfig `json:"onlineStatus"`
	OfflineStatus infrared.StatusConfig `json:"offlineStatus"`
}

func NewServerInfo(cfg ServerConfig) ServerInfo {
	defaultCfg := DefaultServerConfig()
	defaultCfg.updateServerConfig(cfg)
	info := ServerInfo{
		Cfg:     &defaultCfg,
		CloseCh: make(chan struct{}),
		ConnCh:  make(chan connection.HandshakeConn),
	}
	return info
}

type ServerInfo struct {
	Cfg         *ServerConfig

	CloseCh chan struct{}
	ConnCh  chan connection.HandshakeConn

	errLogger func(err error)
	logger    func(a ...interface{})
}

func NewProxyLaneConfig() ProxyLaneConfig {
	return ProxyLaneConfig{
		ReceiveProxyProtocol: false,
		Timeout:              1000,
		ListenTo:             ":25565",

		ListenerFactory: func(addr string) (net.Listener, error) {
			return net.Listen("tcp", addr)
		},
		ErrLogger: func(err error) {
			log.Println(err)
		},
		Logger: func(a ...interface{}) {
			log.Println(a...)
		},
	}
}

type ProxyLaneConfig struct {
	ReceiveProxyProtocol bool   `json:"receiveProxyProtocol"`
	Timeout              int    `json:"timeout"`
	ListenTo             string `json:"listenTo"`

	Servers       []ServerConfig  `json:"servers"`
	DefaultStatus protocol.Packet `json:"defaultStatus"`

	// Seperate this so we can test without making actual network calls
	// ServerConnFactory connection.NewServerConnFactory
	ListenerFactory gateway.ListenerFactory
	ErrLogger       func(err error)
	Logger          func(a ...interface{})
}

func NewProxyLane(cfg ProxyLaneConfig) ProxyLane {
	return ProxyLane{
		listenTo:        cfg.ListenTo,
		listenerFactory: cfg.ListenerFactory,

		errLogger: cfg.ErrLogger,
		logger:    cfg.Logger,

		config:      cfg,
		toGatewayCh: make(chan connection.HandshakeConn),
		gwCloseCh:   make(chan struct{}),
		serverMap:   make(map[string]ServerInfo),
	}
}

type ProxyLane struct {
	config          ProxyLaneConfig
	listenTo        string
	listenerFactory gateway.ListenerFactory
	connFactory     connection.ServerConnFactory

	errLogger func(err error)
	logger    func(a ...interface{})

	isGatewayActive bool
	toGatewayCh     chan connection.HandshakeConn
	gwCloseCh       chan struct{}
	serverMap       map[string]ServerInfo
}

func (proxy *ProxyLane) StartProxy() {
	proxy.gatewayModified()
	proxy.listenerModified()
	proxy.RegisterServers(proxy.config.Servers...)
}

func (proxy *ProxyLane) listenerModified() {
	listener, _ := proxy.listenerFactory(proxy.listenTo)
	l := gateway.NewBasicListener(listener, proxy.toGatewayCh)
	go func() {
		l.Listen()
	}()
}

// TODO: Some error here with already used domains, but it also needs to check extradomains
func (proxy *ProxyLane) RegisterServers(cfgs ...ServerConfig) error {
	for _, cfg := range cfgs {
		serverInfo := NewServerInfo(cfg)
		serverInfo.logger = proxy.logger
		serverInfo.errLogger = proxy.errLogger
		domainName := cfg.MainDomain
		proxy.serverMap[domainName] = serverInfo
		serverInfo.runMCServer()
	}
	proxy.gatewayModified()
	return nil
}

func (proxy *ProxyLane) CloseServer(mainDomain string) {
	serverInfo := proxy.serverMap[mainDomain]
	serverInfo.CloseCh <- struct{}{}
	delete(proxy.serverMap, mainDomain)
	proxy.gatewayModified()
}

func (proxy *ProxyLane) gatewayModified() {
	serverStore := gateway.CreateDefaultServerStore()
	for _, serverInfo := range proxy.serverMap {
		serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
		serverStore.AddServer(serverInfo.Cfg.MainDomain, serverData)
		for _, subdomain := range serverInfo.Cfg.ExtraDomains {
			serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
			serverStore.AddServer(subdomain, serverData)
		}
	}

	if proxy.isGatewayActive {
		proxy.gwCloseCh <- struct{}{}
	}

	gw := gateway.NewBasicGatewayWithStore(&serverStore, proxy.toGatewayCh, proxy.gwCloseCh)
	go func() {
		gw.Start()
	}()
	proxy.isGatewayActive = true
}

// This will check or the current server and the new configs are change
//  and will apply those changes to the server
func (proxy *ProxyLane) UpdateServer(cfg ServerConfig) {
	serverInfo := proxy.serverMap[cfg.MainDomain]
	var reconstructGateways bool

	// We will for now assume that the mainDomain wont change
	domainMap := make(map[string]int)
	for _, domain := range serverInfo.Cfg.ExtraDomains {
		domainMap[domain] += 1
	}
	for _, domain := range cfg.ExtraDomains {
		domainMap[domain] += 2
	}
	for _, number := range domainMap {
		switch number {
		case 3:
			continue
		case 2, 1: // There is a domain removed or added
			reconstructGateways = true
			break
		}
	}

	err := serverInfo.Cfg.updateServerConfig(cfg)
	if err != nil {
		proxy.errLogger(err)
		return
	}
	proxy.serverMap[cfg.MainDomain] = serverInfo
	serverInfo.CloseCh <- struct{}{}
	serverInfo.runMCServer()

	if reconstructGateways {
		proxy.gatewayModified()
	}
}

func (info ServerInfo) runMCServer() {
	playersConnected.WithLabelValues(info.Cfg.MainDomain)
	actionsJoining := []func(){
		func() {
			playersConnected.With(prometheus.Labels{"host": info.Cfg.MainDomain}).Inc()
		},
	}

	actionsLeaving := []func(){
		func() {
			playersConnected.With(prometheus.Labels{"host": info.Cfg.MainDomain}).Dec()
		},
	}

	var onlineStatus, offlineStatus protocol.Packet
	// TODO: Should look into doing this differently, this check
	if info.Cfg.OnlineStatus.ProtocolNumber > 0 {
		onlineStatus, _ = info.Cfg.OnlineStatus.StatusResponsePacket()
	}
	if info.Cfg.OfflineStatus.ProtocolNumber > 0 {
		offlineStatus, _ = info.Cfg.OfflineStatus.StatusResponsePacket()
	}

	timeout := time.Duration(info.Cfg.DialTimeout) * time.Millisecond
	dialer := net.Dialer{
		Timeout: time.Millisecond * time.Duration(timeout),
		LocalAddr: &net.TCPAddr{
			IP: net.ParseIP(info.Cfg.ProxyBind),
		},
	}
	connFactory := func() (connection.ServerConn, error) {
		c, err := dialer.Dial("tcp", info.Cfg.ProxyTo)
		if err != nil {
			info.errLogger(err)
			return connection.ServerConn{}, err
		}
		return connection.NewServerConn(c), nil
	}

	mcServer := server.MCServer{
		CreateServerConn:    connFactory,
		SendProxyProtocol:   info.Cfg.SendProxyProtocol,
		RealIP:              info.Cfg.RealIP,
		OfflineConfigStatus: offlineStatus,
		OnlineConfigStatus:  onlineStatus,
		ConnCh:              info.ConnCh,
		CloseCh:             info.CloseCh,
		JoiningActions:      actionsJoining,
		LeavingActions:      actionsLeaving,
	}

	go func(server server.MCServer) {
		server.Start()
	}(mcServer)
}

func (cfg *ServerConfig) updateServerConfig(newCfg ServerConfig) error {
	var defaultCfg map[string]interface{}
	bb, err := json.Marshal(DefaultServerConfig())
	err = json.Unmarshal(bb, &defaultCfg)

	var loadedCfg map[string]interface{}
	bb, err = json.Marshal(newCfg)
	err = json.Unmarshal(bb, &loadedCfg)

	for k, v := range loadedCfg {
		defaultCfg[k] = v
	}

	bb, err = json.Marshal(defaultCfg)
	err = json.Unmarshal(bb, cfg)
	return err
}

// This methed is meant for testing only usage
func (proxy *ProxyLane) TestMethod_ServerMap() map[string]ServerInfo {
	return proxy.serverMap
}

// This methed is meant for testing only usage
func (proxy *ProxyLane) TestMethod_GatewayCh() (chan connection.HandshakeConn, chan struct{}) {
	return proxy.toGatewayCh, proxy.gwCloseCh
}
