package proxy

import (
	"net"

	"github.com/haveachin/infrared/config"
	"github.com/haveachin/infrared/gateway"
)

type ProxyLaneManager struct {
	// the key here is the listen address of the proxylane
	Proxies map[string]*ProxyLane
	ListenerFactory gateway.ListenerFactory
}

func NewProxyLaneManager() ProxyLaneManager {
	return ProxyLaneManager{
		Proxies: make(map[string]*ProxyLane),
		ListenerFactory: func(addr string) (net.Listener, error) {
			return net.Listen("tcp", addr)
		},
	}
}

func (m *ProxyLaneManager) AddServer(serverCfgs ...config.ServerConfig) error {
	for _, cfg := range serverCfgs {
		proxylane := m.proxylane(cfg.ListenTo)
		proxylane.RegisterServers(cfg)
	}

	return nil
}

func (m *ProxyLaneManager) proxylane(listenAddr string) *ProxyLane {
	if _, ok := m.Proxies[listenAddr]; !ok {
		proxylaneCfg := NewProxyLaneConfig()
		proxylaneCfg.ListenTo = listenAddr
		proxylaneCfg.ListenerFactory = m.ListenerFactory
		proxyLane := NewProxyLane(proxylaneCfg)
		proxyLane.StartProxy()
		m.Proxies[listenAddr] = &proxyLane
	}
	return m.Proxies[listenAddr]
}
