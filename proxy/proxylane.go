package proxy

import (
	"errors"
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
	ErrNoServerConnImplemented = errors.New("no server connection factory is implemented")
	playersConnected           = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players",
	}, []string{"host"})
)

func NewServerInfo(cfg server.ServerConfig, connFactory connection.ServerConnFactory) ServerInfo {
	return ServerInfo{
		MainDomain:        cfg.MainDomain,
		ExtraDomains:      cfg.ExtraDomains,
		NumberOfInstances: cfg.NumberOfInstances,

		Cfg:         cfg,
		ConnFactory: connFactory,

		CloseCh: make(chan struct{}),
		ConnCh:  make(chan connection.HandshakeConn),
	}
}

type ServerInfo struct {
	MainDomain        string
	ExtraDomains      []string
	NumberOfInstances int

	// Look into this later (this needs to change)
	Cfg         server.ServerConfig
	ConnFactory connection.ServerConnFactory

	CloseCh chan struct{}
	ConnCh  chan connection.HandshakeConn
}

type ProxyLaneConfig struct {
	NumberOfListeners int `json:"numberOfListeners"`
	NumberOfGateways  int `json:"numberOfGateways"`

	ReceiveProxyProtocol bool   `json:"receiveProxyProtocol"`
	Timeout              int    `json:"timeout"`
	ListenTo             string `json:"listenTo"`

	Servers       []server.ServerConfig `json:"servers"`
	DefaultStatus protocol.Packet       `json:"defaultStatus"`

	// Seperate this so we can test without making actual network calls
	ServerConnFactory connection.NewServerConnFactory
	ListenerFactory   gateway.ListenerFactory
}

func NewProxyLane(cfg ProxyLaneConfig) ProxyLane {
	return ProxyLane{
		config: cfg,

		toGatewayCh: make(chan connection.HandshakeConn),
		gwCloseCh:   make(chan struct{}),
		serverMap:   make(map[string]ServerInfo),

		connFactory: func(addr string) (connection.ServerConn, error) {
			return connection.ServerConn{}, ErrNoServerConnImplemented
		},
	}
}

type ProxyLane struct {
	config      ProxyLaneConfig
	connFactory connection.ServerConnFactory

	activeGateways int
	toGatewayCh    chan connection.HandshakeConn
	gwCloseCh      chan struct{}
	serverMap      map[string]ServerInfo
}

func (proxy *ProxyLane) StartupProxy() {
	if proxy.config.ServerConnFactory != nil {
		timeout := time.Duration(proxy.config.Timeout) * time.Millisecond
		proxy.connFactory, _ = proxy.config.ServerConnFactory(timeout)
	}

	if proxy.config.NumberOfGateways > 0 {
		proxy.gatewayModified()
	}
	if proxy.config.NumberOfListeners > 0 {
		proxy.listenerModified()
	}
	if len(proxy.config.Servers) > 0 {
		proxy.RegisterServers(proxy.config.Servers...)
	}
}

// To increase the number of running listeners, decreasing isnt supported yet
func (proxy *ProxyLane) listenerModified() {
	listener, _ := proxy.config.ListenerFactory(proxy.config.ListenTo)
	for i := 0; i < proxy.config.NumberOfListeners; i++ {
		l := gateway.NewBasicListener(listener, proxy.toGatewayCh)
		go func() {
			l.Listen()
		}()
	}
}

// TODO: Some error here with already used domains, but it also needs to check extradomains
func (proxy *ProxyLane) RegisterServers(cfgs ...server.ServerConfig) error {
	for _, cfg := range cfgs {
		serverInfo := NewServerInfo(cfg, proxy.connFactory)
		domainName := cfg.MainDomain
		proxy.serverMap[domainName] = serverInfo
		serverInfo.startAllInstances()
	}

	proxy.gatewayModified()
	return nil
}

func (proxy *ProxyLane) CloseServer(mainDomain string) {
	// We should look into transferring them to a different proxylane in the future
	serverInfo := proxy.serverMap[mainDomain]
	for i := 0; i < serverInfo.NumberOfInstances; i++ {
		serverInfo.CloseCh <- struct{}{}
	}

	// Not sure or closing these will have any effect
	close(serverInfo.CloseCh)
	close(serverInfo.ConnCh)
	delete(proxy.serverMap, mainDomain)

	proxy.gatewayModified()
}

func (proxy *ProxyLane) gatewayModified() {
	serverStore := gateway.CreateDefaultServerStore()
	for _, serverInfo := range proxy.serverMap {
		serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
		serverStore.AddServer(serverInfo.MainDomain, serverData)
		for _, subdomain := range serverInfo.ExtraDomains {
			serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
			serverStore.AddServer(subdomain, serverData)
		}
	}

	//Close them all before we start or new ones, so we dont accidently end up closing a new gateway
	for i := 0; i < proxy.activeGateways; i++ {
		proxy.gwCloseCh <- struct{}{}
	}

	for i := 0; i < proxy.config.NumberOfGateways; i++ {
		gw := gateway.NewBasicGatewayWithStore(&serverStore, proxy.toGatewayCh, proxy.gwCloseCh)
		go func() {
			gw.Start()
		}()
	}
	proxy.activeGateways = proxy.config.NumberOfGateways
}

// This will check or the current server and the new configs are change
//  and will apply those changes to the server
func (proxy *ProxyLane) UpdateServer(cfg server.ServerConfig) {
	serverInfo := proxy.serverMap[cfg.MainDomain]
	var hasDifferentDomains bool
	var reconstructGateways bool
	var createNewConnFactory bool
	var needToRestartAllServers bool

	// We will for now assume that the mainDomain wont change
	domainMap := make(map[string]int)
	for _, domain := range serverInfo.ExtraDomains {
		domainMap[domain] += 1
	}
	for _, domain := range cfg.ExtraDomains {
		domainMap[domain] += 2
	}
	for _, number := range domainMap {
		switch number {
		case 3:
			continue
		case 2, 1:
			hasDifferentDomains = true
			break
		}
	}
	if hasDifferentDomains {
		serverInfo.ExtraDomains = cfg.ExtraDomains
		reconstructGateways = true
	}

	// only one of these needs to be truth for all changes to apply
	if serverInfo.Cfg.ProxyBind != cfg.ProxyBind {
		createNewConnFactory = true
		needToRestartAllServers = true
	} else if serverInfo.Cfg.SendProxyProtocol != cfg.SendProxyProtocol {
		createNewConnFactory = true // TODO: Not sure about this one
		needToRestartAllServers = true
	} else if serverInfo.Cfg.ProxyTo != cfg.ProxyTo {
		createNewConnFactory = true
		needToRestartAllServers = true
	} else if serverInfo.Cfg.RealIP != cfg.RealIP {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.DialTimeout != cfg.DialTimeout {
		createNewConnFactory = true
		needToRestartAllServers = true
	} else if serverInfo.Cfg.DisconnectMessage != cfg.DisconnectMessage {
		needToRestartAllServers = true
	} else {
		if same, _ := infrared.SameStatus(serverInfo.Cfg.OnlineStatus, cfg.OnlineStatus); !same {
			needToRestartAllServers = true
		}
		if same, _ := infrared.SameStatus(serverInfo.Cfg.OfflineStatus, cfg.OfflineStatus); !same {
			needToRestartAllServers = true
		}
	}

	serverInfo.Cfg = cfg
	if needToRestartAllServers {
		if createNewConnFactory {
			// Create new conn factory here
		}

		for i := 0; i < serverInfo.NumberOfInstances; i++ {
			serverInfo.CloseCh <- struct{}{}
		}
		serverInfo.startAllInstances()
	} else if serverInfo.NumberOfInstances != cfg.NumberOfInstances {
		adjustNumber := cfg.NumberOfInstances - serverInfo.NumberOfInstances
		for adjustNumber != 0 {
			if adjustNumber > 0 {
				mcServer := serverInfo.createMCServer()
				go func(server server.MCServer) {
					server.Start()
				}(mcServer)
				adjustNumber--
			} else {
				serverInfo.CloseCh <- struct{}{}
				adjustNumber++
			}
		}
		serverInfo.NumberOfInstances = cfg.NumberOfInstances
	}

	proxy.serverMap[cfg.MainDomain] = serverInfo
	if reconstructGateways {
		proxy.gatewayModified()
	}
}

// This create all instances of a server
func (serverInfo ServerInfo) startAllInstances() {
	mcServer := serverInfo.createMCServer()
	for i := 0; i < serverInfo.NumberOfInstances; i++ {
		go func(server server.MCServer) {
			// With this every server will be a unique instance
			server.Start()
		}(mcServer)
	}
}

func (info ServerInfo) createMCServer() server.MCServer {
	playersConnected.WithLabelValues(info.MainDomain)
	actionsJoining := []func(domain string){
		func(domain string) {
			playersConnected.With(prometheus.Labels{"host": domain}).Inc()
		},
	}

	actionsLeaving := []func(domain string){
		func(domain string) {
			playersConnected.With(prometheus.Labels{"host": domain}).Dec()
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

	return server.MCServer{
		Config:              info.Cfg,
		ConnFactory:         info.ConnFactory,
		OnlineConfigStatus:  onlineStatus,
		OfflineConfigStatus: offlineStatus,

		ConnCh:  info.ConnCh,
		CloseCh: info.CloseCh,

		JoiningActions: actionsJoining,
		LeavingActions: actionsLeaving,
	}
}

// This methed is meant for testing only usage
func (proxy *ProxyLane) TestMethod_ServerMap() map[string]ServerInfo {
	return proxy.serverMap
}

// This methed is meant for testing only usage
func (proxy *ProxyLane) TestMethod_GatewayCh() (chan connection.HandshakeConn, chan struct{}) {
	return proxy.toGatewayCh, proxy.gwCloseCh
}
