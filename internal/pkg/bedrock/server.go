package bedrock

import (
	"net"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/pires/go-proxyproto"
	"github.com/sandertv/go-raknet"
)

type Server struct {
	ID                 string
	Domains            []string
	Dialer             raknet.Dialer
	Address            string
	SendProxyProtocol  bool
	DialTimeoutMessage string
	GatewayIDs         []string
	WebhookIDs         []string
}

type InfraredServer struct {
	Server
}

func (s InfraredServer) Edition() infrared.Edition {
	return infrared.BedrockEdition
}

func (s InfraredServer) ID() string {
	return s.Server.ID
}

func (s InfraredServer) Domains() []string {
	return s.Server.Domains
}

func (s InfraredServer) GatewayIDs() []string {
	return s.Server.GatewayIDs
}

func (s InfraredServer) WebhookIDs() []string {
	return s.Server.WebhookIDs
}

func (s InfraredServer) Dial() (*Conn, error) {
	c, err := s.Dialer.Dial(s.Address)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn: c,
	}, nil
}

func (s InfraredServer) HandleConn(c net.Conn) (infrared.Conn, error) {
	pc := c.(*ProcessedConn)

	if s.SendProxyProtocol {
		s.Dialer.UpstreamDialer = &proxyProtocolDialer{
			connAddr:       c.RemoteAddr(),
			upstreamDialer: s.Dialer.UpstreamDialer,
		}
	}

	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(*pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if _, err := rc.Write(pc.readBytes); err != nil {
		rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s InfraredServer) handleDialTimeout(c ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.DialTimeoutMessage, c, &s)
	return c.disconnect(msg)
}

type proxyProtocolDialer struct {
	connAddr       net.Addr
	upstreamDialer raknet.UpstreamDialer
}

func (d proxyProtocolDialer) Dial(network, address string) (net.Conn, error) {
	rc, err := d.upstreamDialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	if err := writeProxyProtocolHeader(d.connAddr, rc); err != nil {
		rc.Close()
		return nil, err
	}

	return rc, nil
}

func writeProxyProtocolHeader(connAddr net.Addr, rc net.Conn) error {
	tp := proxyproto.UDPv4
	addr := connAddr.(*net.UDPAddr)
	if addr.IP.To4() == nil {
		tp = proxyproto.UDPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        connAddr,
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}

	return nil
}
