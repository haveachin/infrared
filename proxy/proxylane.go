package proxy

import (
	"net"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/server"
)

type ProxyLaneConfig struct {
	NumberOfListeners int `json:"numberOfListeners"`
	NumberOfGateways  int `json:"numberOfGateways"`
	// ProxyProtocol     bool   `json:"proxyProtocol"`
	Timeout  int    `json:"timeout"`
	ListenTo string `json:"listenTo"`

	Servers []server.ServerConfig
}

type ProxyLane struct {
	Config ProxyLaneConfig

	toGatewayChan chan connection.HandshakeConn
	toServerChan  map[string]chan connection.HandshakeConn
}

func (proxy *ProxyLane) StartupProxy() {
	proxy.toGatewayChan = make(chan connection.HandshakeConn)
	proxy.toServerChan = make(map[string]chan connection.HandshakeConn)

	serverStore := &gateway.DefaultServerStore{}
	for i := 0; i < len(proxy.Config.Servers); i++ {
		domainName := proxy.Config.Servers[i].DomainName

		serverCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: serverCh}
		serverStore.AddServer(domainName, serverData)
		proxy.toServerChan[domainName] = serverCh
	}

	//Listener
	outerListener := gateway.NewBasicOuterListener(proxy.Config.ListenTo)
	for i := 0; i < proxy.Config.NumberOfListeners; i++ {
		l := gateway.BasicListener{OutListener: outerListener, ConnCh: proxy.toGatewayChan}

		go func() {
			l.Listen()
		}()
	}

	//Gateway

	for i := 0; i < proxy.Config.NumberOfGateways; i++ {
		gw := gateway.NewBasicGatewayWithStore(serverStore, proxy.toGatewayChan)
		go func() {
			gw.Start()
		}()
	}

	if proxy.Config.Timeout == 0 {
		proxy.Config.Timeout = 250
	}
	dialTimeoutTime := time.Duration(proxy.Config.Timeout) * time.Millisecond

	//Server
	connFactory := func(addr string) (connection.ServerConn, error) {
		c, err := net.DialTimeout("tcp", addr, dialTimeoutTime)
		if err != nil {
			return nil, err
		}
		return connection.NewBasicServerConn(c), nil
	}

	for i := 0; i < len(proxy.Config.Servers); i++ {
		cfg := proxy.Config.Servers[i]

		makeServer := func(cfg server.ServerConfig, sConnFactory func(addr string) (connection.ServerConn, error)) server.MCServer {
			onlineStatus, _ := cfg.OnlineStatus.StatusResponsePacket()
			offlineStatus, _ := cfg.OfflineStatus.StatusResponsePacket()
			serverCh := proxy.toServerChan[cfg.DomainName]
			connFac := connFactory
			return server.MCServer{
				Config:              cfg,
				ConnFactory:         connFac,
				OnlineConfigStatus:  onlineStatus,
				OfflineConfigStatus: offlineStatus,
				ConnCh:              serverCh,
			}
		}
		mcServer := makeServer(cfg, connFactory)
		for i := 0; i < cfg.NumberOfInstances; i++ {
			go func(server server.Server) {
				// With this ever server should be a unique instance in the goroutine
				server.Start()
			}(&mcServer)
		}

	}

}
