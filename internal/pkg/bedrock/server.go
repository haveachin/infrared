package bedrock

import (
	"errors"
	"fmt"
	"net"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
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

func (s InfraredServer) Dial() (*Conn, error) {
	c, err := s.Dialer.Dial(s.Address)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn:    c,
		decoder: packet.NewDecoder(c),
		encoder: packet.NewEncoder(c),
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

	if pc.requestNetworkSettingsPkData != nil {
		if err := rc.encoder.Encode(pc.requestNetworkSettingsPkData.Full); err != nil {
			rc.Close()
			return nil, err
		}

		pksData, err := rc.ReadPackets()
		if err != nil {
			rc.Close()
			return nil, err
		}

		if len(pksData) < 1 {
			rc.Close()
			return nil, fmt.Errorf("invalid amount of packets received: expected <1; got %d", len(pksData))
		}

		var networkSettingsPk packet.NetworkSettings
		if err := pksData[0].Decode(&networkSettingsPk); err != nil {
			rc.Close()
			return nil, err
		}

		if networkSettingsPk.CompressionAlgorithm != pc.compression {
			rc.Close()
			return nil, errors.New("server compression does not match")
		}
		rc.EnableCompression(networkSettingsPk.CompressionAlgorithm)

		for i := 1; i < len(pksData); i++ {
			if err := pc.encoder.Encode(pksData[i].Full); err != nil {
				rc.Close()
				return nil, err
			}
		}
	}

	if err := rc.encoder.Encode(pc.loginPkData.Full); err != nil {
		rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s InfraredServer) handleDialTimeout(c ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.DialTimeoutMessage, &c, &s)
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
