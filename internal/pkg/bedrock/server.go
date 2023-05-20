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
	id                      string
	domains                 []string
	dialer                  raknet.Dialer
	address                 string
	sendProxyProtocol       bool
	dialTimeoutDisconnecter *PlayerDisconnecter
	gatewayIDs              []string
}

func (s Server) Edition() infrared.Edition {
	return infrared.BedrockEdition
}

func (s Server) ID() string {
	return s.id
}

func (s Server) Domains() []string {
	return s.domains
}

func (s Server) GatewayIDs() []string {
	return s.gatewayIDs
}

func (s Server) Dial() (*Conn, error) {
	c, err := s.dialer.Dial(s.address)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn:    c,
		decoder: packet.NewDecoder(c),
		encoder: packet.NewEncoder(c),
	}, nil
}

func (s Server) NewConn(p infrared.Conn) (infrared.Conn, error) {
	player := p.(*Player)
	player.serverID = s.id

	if s.sendProxyProtocol {
		s.dialer.UpstreamDialer = &proxyProtocolDialer{
			connAddr:       player.RemoteAddr(),
			upstreamDialer: s.dialer.UpstreamDialer,
		}
	}

	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(player); err != nil {
			return nil, err
		}
		return nil, err
	}

	if player.requestNetworkSettingsPkData != nil {
		if err := rc.encoder.Encode(player.requestNetworkSettingsPkData.Full); err != nil {
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

		if networkSettingsPk.CompressionAlgorithm != player.compression {
			rc.Close()
			return nil, errors.New("server compression does not match")
		}
		rc.EnableCompression(networkSettingsPk.CompressionAlgorithm)

		for i := 1; i < len(pksData); i++ {
			if err := player.encoder.Encode(pksData[i].Full); err != nil {
				rc.Close()
				return nil, err
			}
		}
	}

	if err := rc.encoder.Encode(player.loginPkData.Full); err != nil {
		rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s Server) handleDialTimeout(p *Player) error {
	return s.dialTimeoutDisconnecter.DisconnectPlayer(p, infrared.ApplyTemplates(
		infrared.TimeMessageTemplates(),
		infrared.PlayerMessageTemplates(p),
		infrared.ServerMessageTemplate(s),
	))
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
