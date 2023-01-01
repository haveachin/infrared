package java

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/pires/go-proxyproto"
	"golang.org/x/net/proxy"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type statusResponseJSONProvider struct {
	server *Server

	mu                      sync.Mutex
	cacheTTL                time.Duration
	cacheSetAt              time.Time
	statusResponseJSONCache status.ResponseJSON
}

func (s *statusResponseJSONProvider) isStatusCacheValid() bool {
	return s.cacheTTL > 0 && s.cacheSetAt.Add(s.cacheTTL).After(time.Now())
}

func (s *statusResponseJSONProvider) requestNewStatusResponseJSON(p *Player) (status.ResponseJSON, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return status.ResponseJSON{}, err
	}

	if err := s.server.prepareConns(p, rc); err != nil {
		rc.Close()
		return status.ResponseJSON{}, err
	}

	if err := rc.WritePackets(p.readPks...); err != nil {
		return status.ResponseJSON{}, err
	}

	pk, err := rc.ReadPacket(status.MaxSizeClientBoundResponse)
	if err != nil {
		return status.ResponseJSON{}, err
	}
	rc.Close()

	respPk, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return status.ResponseJSON{}, err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return status.ResponseJSON{}, err
	}

	return respJSON, nil
}

func (s *statusResponseJSONProvider) StatusResponseJSON(p *Player) (status.ResponseJSON, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isStatusCacheValid() {
		return s.statusResponseJSONCache, nil
	}

	respJSON, err := s.requestNewStatusResponseJSON(p)
	if err != nil {
		return status.ResponseJSON{}, err
	}

	s.statusResponseJSONCache = respJSON
	s.cacheSetAt = time.Now()

	return respJSON, nil
}

type StatusResponseJSONProvider interface {
	StatusResponseJSON(p *Player) (status.ResponseJSON, error)
}

type Server struct {
	id                         string
	domains                    []string
	addr                       string
	addrHost                   string
	addrPort                   int
	sendProxyProtocol          bool
	sendRealIP                 bool
	overrideAddress            bool
	dialer                     proxy.Dialer
	overrideStatus             OverrideServerStatusResponse
	dialTimeoutDisconnector    *PlayerDisconnecter
	gatewayIDs                 []string
	statusResponseJSONProvider StatusResponseJSONProvider
}

func (s Server) Edition() infrared.Edition {
	return infrared.JavaEdition
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
	c, err := s.dialer.Dial("tcp", s.addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}, nil
}

func (s Server) HandleConn(c net.Conn) (infrared.Conn, error) {
	pc := c.(*Player)
	if pc.handshake.IsStatusRequest() {
		if err := s.handleStatusPing(pc); err != nil {
			return nil, err
		}
		return nil, infrared.ErrClientStatusRequest
	}

	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if err := s.prepareConns(pc, rc); err != nil {
		rc.Close()
		return nil, err
	}

	// Sends the handshake and the request or login packet to the server
	if err := rc.WritePackets(pc.readPks...); err != nil {
		rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s Server) prepareConns(p *Player, rc net.Conn) error {
	if s.sendProxyProtocol {
		if err := writeProxyProtocolHeader(p.RemoteAddr(), rc); err != nil {
			return err
		}
	}

	if s.sendRealIP {
		p.handshake.UpgradeToRealIP(p.RemoteAddr(), time.Now())
		p.readPks[0] = p.handshake.Marshal()
	}

	if s.overrideAddress {
		p.handshake.SetServerAddress(s.addrHost)
		p.handshake.ServerPort = protocol.UnsignedShort(s.addrPort)
		p.readPks[0] = p.handshake.Marshal()
	}

	return nil
}

func (s Server) handleDialTimeout(p *Player) error {
	if err := s.dialTimeoutDisconnector.DisconnectPlayer(p); err != nil {
		return err
	}

	if p.handshake.IsStatusRequest() {
		return infrared.ErrClientStatusRequest
	}

	return nil
}

func (s Server) handleStatusPing(p *Player) error {
	if err := s.overrideStatusResponse(p); err != nil {
		return err
	}

	ping, err := p.ReadPacket(status.MaxSizeServerBoundPingRequest)
	if err != nil {
		return err
	}

	return p.WritePacket(ping)
}

func (s Server) overrideStatusResponse(p *Player) error {
	respJSON, err := s.statusResponseJSONProvider.StatusResponseJSON(p)
	if err != nil {
		return err
	}

	respJSON = s.overrideStatus.ResponseJSON(respJSON, MOTDOption(infrared.ApplyTemplates(
		infrared.TimeMessageTemplates(),
		infrared.PlayerMessageTemplates(p),
		infrared.ServerMessageTemplate(s),
	)))

	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	return p.WritePacket(status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal())
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	tp := proxyproto.TCPv4
	tcpAddr := addr.(*net.TCPAddr)
	if tcpAddr.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        addr,
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}
