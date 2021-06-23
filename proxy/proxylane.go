package proxy

import (
	"fmt"
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

func NewServerInfo(cfg server.ServerConfig, connFactory connection.ServerConnFactory) ServerInfo {
	proxiesActive.Inc()
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

	// Look into this later
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

type ProxyLane struct {
	Config ProxyLaneConfig

	connFactory connection.ServerConnFactory

	toGatewayCh chan connection.HandshakeConn
	gwCloseCh   chan struct{}
	serverMap   map[string]ServerInfo
}

func (proxy *ProxyLane) StartupProxy() {
	proxy.toGatewayCh = make(chan connection.HandshakeConn)

	servers := proxy.Config.Servers

	timeout := time.Duration(proxy.Config.Timeout) * time.Millisecond
	proxy.connFactory, _ = proxy.Config.ServerConnFactory(timeout)

	proxy.RegisterMultipleServers(servers)
	proxy.HandleGateways(proxy.toGatewayCh)
	proxy.HandleListeners(proxy.toGatewayCh)

	for _, server := range servers {
		proxiesActive.Inc()
		proxy.InitialServerSetup(server)
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
	for _, serverInfo := range proxy.serverMap {
		serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
		serverStore.AddServer(serverInfo.MainDomain, serverData)
		for _, subdomain := range serverInfo.ExtraDomains {
			serverData := gateway.ServerData{ConnCh: serverInfo.ConnCh}
			serverStore.AddServer(subdomain, serverData)
		}
	}

	for i := 0; i < proxy.Config.NumberOfGateways; i++ {
		gw := gateway.NewBasicGatewayWithStore(&serverStore, gatewayCh, proxy.gwCloseCh)
		go func() {
			gw.Start()
		}()
	}
}

func (proxy *ProxyLane) RegisterMultipleServers(servers []server.ServerConfig) {
	if proxy.serverMap == nil {
		proxy.serverMap = make(map[string]ServerInfo)
	}

	for _, serverCfg := range servers {
		serverInfo := NewServerInfo(serverCfg, proxy.connFactory)
		domainName := serverCfg.MainDomain
		proxy.serverMap[domainName] = serverInfo
	}
}

func (proxy *ProxyLane) InitialServerSetup(cfg server.ServerConfig) {
	serverInfo := proxy.serverMap[cfg.MainDomain]
	mcServer := serverInfo.createMCServer()
	for i := 0; i < cfg.NumberOfInstances; i++ {
		go func(server server.MCServer) {
			// With this every server will be a unique instance
			server.Start()
		}(mcServer)
	}
}

func (proxy *ProxyLane) RegisterSingleServer(cfg server.ServerConfig) {
	if proxy.serverMap == nil {
		proxy.serverMap = make(map[string]ServerInfo)
	}

	serverInfo := NewServerInfo(cfg, proxy.connFactory)
	domainName := cfg.MainDomain
	proxy.serverMap[domainName] = serverInfo

	proxy.InitialServerSetup(cfg)

	// Change gateway store...
	proxy.gatewayServersModified()

}

func (proxy *ProxyLane) CloseServer(mainDomain string) {
	// Get all running instances and close them...?
	// We should look into transferring them to a different proxylane in the future
	serverInfo := proxy.serverMap[mainDomain]
	for i := 0; i < serverInfo.NumberOfInstances; i++ {
		serverInfo.CloseCh <- struct{}{}
	}

	// Change gateway store...
	proxy.gatewayServersModified()

	// Remove serverInfo
	delete(proxy.serverMap, mainDomain)
	proxiesActive.Dec()
}

func (proxy *ProxyLane) gatewayServersModified() {
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
	for i := 0; i < proxy.Config.NumberOfGateways; i++ {
		proxy.gwCloseCh <- struct{}{}
	}

	for i := 0; i < proxy.Config.NumberOfGateways; i++ {
		gw := gateway.NewBasicGatewayWithStore(&serverStore, proxy.toGatewayCh, proxy.gwCloseCh)
		go func() {
			gw.Start()
		}()
	}

}

// This will check or the current server and the new configs are change
//  and will apply those changes to the server
// Yes better method name is needed
func (proxy *ProxyLane) UpdateServer(cfg server.ServerConfig) {
	serverInfo := proxy.serverMap[cfg.MainDomain]
	var reconstructGateways bool
	var needToRestartAllServers bool

	//Check or domains have changed if so, change server store settings
	// We will for now assume that the mainDomain wont change
	newDomainInt := 2
	currentDomainInt := 1
	addedDomains := []string{}
	removedDomains := []string{}
	domainMap := make(map[string]int)
	for _, domain := range serverInfo.ExtraDomains {
		domainMap[domain] += currentDomainInt
	}
	for _, domain := range cfg.ExtraDomains {
		domainMap[domain] += newDomainInt
	}

	for domain, number := range domainMap {
		switch number {
		case newDomainInt + currentDomainInt:
			continue
		case newDomainInt:
			addedDomains = append(addedDomains, domain)
			fmt.Printf("Domain '%s' has been added to proxy '%s'\n", domain, cfg.MainDomain)
		case currentDomainInt:
			removedDomains = append(removedDomains, domain)
			fmt.Printf("Domain '%s' has been removed from proxy '%s'\n", domain, cfg.MainDomain)
		}
	}

	if len(addedDomains) > 0 || len(removedDomains) > 0 {
		reconstructGateways = true
	}

	// only one of these has to be truth for all changes to apply
	if serverInfo.Cfg.ProxyBind != cfg.ProxyBind {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.SendProxyProtocol != cfg.SendProxyProtocol {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.ProxyTo != cfg.ProxyTo {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.RealIP != cfg.RealIP {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.DialTimeout != cfg.DialTimeout {
		needToRestartAllServers = true
	} else if serverInfo.Cfg.DisconnectMessage != cfg.DisconnectMessage {
		needToRestartAllServers = true
	} else {
		sameOnlineStatus, _ := infrared.SameStatus(serverInfo.Cfg.OnlineStatus, cfg.OnlineStatus)
		if sameOnlineStatus {
			needToRestartAllServers = true
		}
		sameOfflineStatus, _ := infrared.SameStatus(serverInfo.Cfg.OfflineStatus, cfg.OfflineStatus)
		if sameOfflineStatus {
			needToRestartAllServers = true
		}
	}

	if needToRestartAllServers {
		for i := 0; i < serverInfo.NumberOfInstances; i++ {
			serverInfo.CloseCh <- struct{}{}
		}
		proxy.InitialServerSetup(cfg)
	}

	// Figure first out or there isnt something critical changed in the cfg
	// that  we dont have to adjust the value and than take them all down
	//  to then create new instances
	if !needToRestartAllServers && serverInfo.NumberOfInstances != cfg.NumberOfInstances {
		// Change here the number of running instances
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
	}

	if reconstructGateways {
		proxy.gatewayServersModified()
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
