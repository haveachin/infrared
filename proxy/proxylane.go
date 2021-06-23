package proxy

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/server"
)

var (
	// There is no closing yet, also some more numberes might be fun..?
	proxiesActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "infrared_proxies",
		Help: "The total number of proxies running",
	})

	playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players",
	}, []string{"host"})
)

func NewServerInfo(cfg server.ServerConfig) ServerInfo {
	return ServerInfo{
		DomainName:        cfg.DomainName,
		SubDomains:        cfg.SubDomains,
		NumberOfInstances: cfg.NumberOfInstances,

		CloseCh: make(chan struct{}),
		ConnCh:  make(chan connection.HandshakeConn),
	}
}

type ServerInfo struct {
	DomainName        string
	SubDomains        []string
	NumberOfInstances int

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

type ProxyLane struct {
	Config ProxyLaneConfig

	connFactory connection.ServerConnFactory

	toGatewayChan chan connection.HandshakeConn
	toServerChans map[string]chan connection.HandshakeConn
}

func (proxy *ProxyLane) StartupProxy() {
	proxy.toGatewayChan = make(chan connection.HandshakeConn)
	proxy.toServerChans = make(map[string]chan connection.HandshakeConn)

	servers := proxy.Config.Servers

	timeout := time.Duration(proxy.Config.Timeout) * time.Millisecond
	proxy.connFactory, _ = proxy.Config.ServerConnFactory(timeout)

	proxy.LoadServers(servers)
	proxy.HandleGateways(proxy.toGatewayChan)
	proxy.HandleListeners(proxy.toGatewayChan)

	for _, server := range servers {
		proxiesActive.Inc()
		proxy.HandleServer(server)
	}

}

func (proxy *ProxyLane) HandleListeners(gatewayCh chan connection.HandshakeConn) {
	listener, _ := proxy.Config.ListenerFactory(proxy.Config.ListenTo)
	for i := 0; i < proxy.Config.NumberOfListeners; i++ {
		l := gateway.NewBasicListener(listener, gatewayCh)
		go func() {
			l.Listen()
		}()
	}
}

func (proxy *ProxyLane) HandleGateways(gatewayCh chan connection.HandshakeConn) {
	serverStore := gateway.CreateDefaultServerStore()
	for url, ch := range proxy.toServerChans {
		serverData := gateway.ServerData{ConnCh: ch}
		serverStore.AddServer(url, serverData)
	}

	for i := 0; i < proxy.Config.NumberOfGateways; i++ {
		gw := gateway.NewBasicGatewayWithStore(&serverStore, gatewayCh, nil)
		go func() {
			gw.Start()
		}()
	}
}

func (proxy *ProxyLane) LoadServers(servers []server.ServerConfig) {
	if proxy.toServerChans == nil {
		proxy.toServerChans = make(map[string]chan connection.HandshakeConn)
	}

	for i := 0; i < len(servers); i++ {
		domainName := servers[i].DomainName
		serverCh := make(chan connection.HandshakeConn)
		proxy.toServerChans[domainName] = serverCh
	}
}

func (proxy *ProxyLane) HandleServer(cfg server.ServerConfig) {
	playersConnected.With(prometheus.Labels{"host": cfg.DomainName}).Dec()
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
	if cfg.OnlineStatus.ProtocolNumber > 0 {
		onlineStatus, _ = cfg.OnlineStatus.StatusResponsePacket()
	}
	if cfg.OfflineStatus.ProtocolNumber > 0 {
		offlineStatus, _ = cfg.OfflineStatus.StatusResponsePacket()
	}

	serverCh := proxy.toServerChans[cfg.DomainName]
	connFac := proxy.connFactory
	mcServer := server.MCServer{
		Config:              cfg,
		ConnFactory:         connFac,
		OnlineConfigStatus:  onlineStatus,
		OfflineConfigStatus: offlineStatus,
		ConnCh:              serverCh,

		JoiningActions: actionsJoining,
		LeavingActions: actionsLeaving,
	}

	for i := 0; i < cfg.NumberOfInstances; i++ {
		go func(server server.MCServer) {
			// With this every server will be a unique instance
			server.Start()
		}(mcServer)
	}

}
