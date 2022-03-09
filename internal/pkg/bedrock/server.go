package bedrock

import (
	"net"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/pires/go-proxyproto"
	"github.com/sandertv/go-raknet"
)

type Server struct {
	ID                 string
	Domains            []string
	Dialer             raknet.Dialer
	DialTimeout        time.Duration
	Address            string
	SendProxyProtocol  bool
	DialTimeoutMessage string
	WebhookIDs         []string
}

type InfraredServer struct {
	Server
}

func (s InfraredServer) ID() string {
	return s.Server.ID
}

func (s InfraredServer) Domains() []string {
	return s.Server.Domains
}

func (s InfraredServer) WebhookIDs() []string {
	return s.Server.WebhookIDs
}

func (s InfraredServer) Dial() (*raknet.Conn, error) {
	c, err := s.Dialer.DialTimeout(s.Address, s.DialTimeout)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s InfraredServer) HandleConn(c net.Conn) (net.Conn, error) {
	pc := c.(*ProcessedConn)
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(*pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if s.SendProxyProtocol {
		if err := writeProxyProtocolHeader(pc, rc); err != nil {
			return nil, err
		}
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

func writeProxyProtocolHeader(c, rc net.Conn) error {
	tp := proxyproto.UDPv4
	addr := c.RemoteAddr().(*net.UDPAddr)
	if addr.IP.To4() == nil {
		tp = proxyproto.UDPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        c.RemoteAddr(),
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}
