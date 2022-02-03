package infrared

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/webhook"
)

type ProxyConfig interface {
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadCPNs() ([]CPN, error)
	LoadWebhooks() ([]webhook.Webhook, error)
	LoadChanCaps() (ProxyChanCaps, error)
}

type ProxyChanCaps struct {
	CPN      int
	Server   int
	ConnPool int
}

type Proxy struct {
	Gateways      []Gateway
	CPNs          []CPN
	ServerGateway ServerGateway
	ConnPool      ConnPool
	ChanCaps      ProxyChanCaps
}

func NewProxy(cfg ProxyConfig) (Proxy, error) {
	gateways, err := cfg.LoadGateways()
	if err != nil {
		return Proxy{}, err
	}

	cpns, err := cfg.LoadCPNs()
	if err != nil {
		return Proxy{}, err
	}

	servers, err := cfg.LoadServers()
	if err != nil {
		return Proxy{}, err
	}

	return Proxy{
		Gateways: gateways,
		CPNs:     cpns,
		ServerGateway: ServerGateway{
			Gateways: gateways,
			Servers:  servers,
		},
		ConnPool: ConnPool{},
	}, nil
}

func (p Proxy) Start(log logr.Logger) error {
	cpnChan := make(chan net.Conn, p.ChanCaps.CPN)
	srvChan := make(chan ProcessedConn, p.ChanCaps.Server)
	poolChan := make(chan ConnTunnel, p.ChanCaps.ConnPool)

	for _, gw := range p.Gateways {
		gw.SetLogger(log)
		go ListenAndServe(gw, cpnChan)
	}

	for _, cpn := range p.CPNs {
		cpn.Log = log
		go cpn.Start(cpnChan, srvChan)
	}

	p.ConnPool.Log = log
	go p.ConnPool.Start(poolChan)

	for _, srv := range p.ServerGateway.Servers {
		srv.SetLogger(log)
	}

	p.ServerGateway.Log = log
	if err := p.ServerGateway.Start(srvChan, poolChan); err != nil {
		return err
	}

	return nil
}
