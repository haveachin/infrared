package infrared

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/webhook"
)

type ProxyConfig interface {
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadCPNs() ([]CPN, error)
	LoadWebhooks() ([]webhook.Webhook, error)
}

type Proxy struct {
	Gateways      []Gateway
	CPNs          []CPN
	ServerGateway ServerGateway
	ConnPool      ConnPool
}

func NewProxy(cfg ProxyConfig) (Proxy, error) {
	gateways, err := cfg.LoadGateways()
	if err != nil {
		return Proxy{}, err
	}

	gwIDsIDs := map[string][]string{}
	srvNotFoundMsgs := map[string]string{}
	for _, gw := range gateways {
		gwIDsIDs[gw.GetID()] = gw.GetServerIDs()
		srvNotFoundMsgs[gw.GetID()] = gw.GetServerNotFoundMessage()
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
			GatewayIDServerIDs:     gwIDsIDs,
			ServerNotFoundMessages: srvNotFoundMsgs,
			Servers:                servers,
		},
		ConnPool: ConnPool{},
	}, nil
}

func (p Proxy) Start(log logr.Logger) error {
	cpnChan := make(chan net.Conn, 10)
	srvChan := make(chan ProcessedConn, 10)
	poolChan := make(chan ConnTunnel, 10)

	for _, gw := range p.Gateways {
		gw.SetLogger(log)
		go gw.ListenAndServe(cpnChan)
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
