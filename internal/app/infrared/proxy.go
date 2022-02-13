package infrared

import (
	"net"

	"github.com/go-logr/logr"
)

type ProxyConfig interface {
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadCPNs() ([]CPN, error)
	LoadChanCaps() (ProxyChanCaps, error)
}

type ProxyChanCaps struct {
	CPN      int
	Server   int
	ConnPool int
}

type Proxy struct {
	gateways      []Gateway
	cpns          []CPN
	serverGateway ServerGateway
	connPool      ConnPool
	cpnCh         chan net.Conn
	srvCh         chan ProcessedConn
	poolCh        chan ConnTunnel
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

	chanCaps, err := cfg.LoadChanCaps()
	if err != nil {
		return Proxy{}, err
	}

	return Proxy{
		gateways: gateways,
		cpns:     cpns,
		serverGateway: ServerGateway{
			Gateways: gateways,
			Servers:  servers,
		},
		connPool: ConnPool{},
		cpnCh:    make(chan net.Conn, chanCaps.CPN),
		srvCh:    make(chan ProcessedConn, chanCaps.Server),
		poolCh:   make(chan ConnTunnel, chanCaps.ConnPool),
	}, nil
}

func (p Proxy) Start(log logr.Logger) {
	for _, gw := range p.gateways {
		gw.SetLogger(log)
		go ListenAndServe(gw, p.cpnCh)
	}

	for i := 0; i < len(p.cpns); i++ {
		cpn := p.cpns[i]
		cpn.Log = log
		cpn.In = p.cpnCh
		cpn.Out = p.srvCh
		go cpn.ListenAndServe()
	}

	p.connPool.Log = log
	go p.connPool.Start(p.poolCh)

	for _, srv := range p.serverGateway.Servers {
		srv.SetLogger(log)
	}

	p.serverGateway.Log = log
	p.serverGateway.Start(p.srvCh, p.poolCh)
}
