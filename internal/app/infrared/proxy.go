package infrared

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

type ProxyConfig interface {
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadCPN() (CPN, error)
	LoadChanCaps() (ProxyChanCaps, error)
}

type ProxyChanCaps struct {
	CPN      int
	Server   int
	ConnPool int
}

type Proxy struct {
	cpnCh         chan net.Conn
	srvCh         chan ProcessedConn
	poolCh        chan ConnTunnel
	gateways      []Gateway
	cpnPool       CPNPool
	serverGateway ServerGateway
	connPool      ConnPool
}

func NewProxy(cfg ProxyConfig) (Proxy, error) {
	gateways, err := cfg.LoadGateways()
	if err != nil {
		return Proxy{}, err
	}

	cpn, err := cfg.LoadCPN()
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
		cpnPool: CPNPool{
			CPN: cpn,
		},
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

	p.cpnPool.CPN.Log = log
	p.cpnPool.CPN.In = p.cpnCh
	p.cpnPool.CPN.Out = p.srvCh
	p.cpnPool.CPN.EventBus = event.DefaultBus
	p.cpnPool.Start(2)

	p.connPool.Log = log
	go p.connPool.Start(p.poolCh)

	for _, srv := range p.serverGateway.Servers {
		srv.SetLogger(log)
	}

	p.serverGateway.Log = log
	p.serverGateway.Start(p.srvCh, p.poolCh)
}
