package java

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/pires/go-proxyproto"
	"golang.org/x/net/proxy"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type statusCacheEntry struct {
	expiresAt      time.Time
	statusResponse status.ResponseJSON
}

func (e statusCacheEntry) isExpired() bool {
	return e.expiresAt.Before(time.Now())
}

type statusResponseJSONProvider struct {
	server *Server

	mu                  sync.Mutex
	cacheTTL            time.Duration
	statusHash          map[protocol.Version]uint64
	statusResponseCache map[uint64]*statusCacheEntry
}

func (s *statusResponseJSONProvider) requestNewStatusResponseJSON(p *Player) (uint64, status.ResponseJSON, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return 0, status.ResponseJSON{}, err
	}

	if err := s.server.prepareConns(p, rc); err != nil {
		rc.Close()
		return 0, status.ResponseJSON{}, err
	}

	if err := rc.WritePackets(p.readPks...); err != nil {
		return 0, status.ResponseJSON{}, err
	}

	pk, err := rc.ReadPacket(status.MaxSizeClientBoundResponse)
	if err != nil {
		return 0, status.ResponseJSON{}, err
	}
	rc.Close()

	hash := xxhash.New()
	hash.Write(pk.Marshal())

	respPk, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return 0, status.ResponseJSON{}, err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return 0, status.ResponseJSON{}, err
	}

	return hash.Sum64(), respJSON, nil
}

func (s *statusResponseJSONProvider) StatusResponseJSON(p *Player) (status.ResponseJSON, error) {
	if s.cacheTTL <= 0 {
		_, statusResp, err := s.requestNewStatusResponseJSON(p)
		return statusResp, err
	}

	// Prunes all expired status reponses
	s.prune()

	s.mu.Lock()
	defer s.mu.Unlock()

	protVer := protocol.Version(p.handshake.ProtocolVersion)
	hash, ok := s.statusHash[protVer]
	if !ok {
		hash, newStatusResp, err := s.requestNewStatusResponseJSON(p)
		if err != nil {
			return status.ResponseJSON{}, err
		}
		s.statusHash[protVer] = hash

		entry, ok := s.statusResponseCache[hash]
		if !ok {
			s.statusResponseCache[hash] = &statusCacheEntry{
				expiresAt:      time.Now().Add(s.cacheTTL),
				statusResponse: newStatusResp,
			}
			return newStatusResp, nil
		}
		return entry.statusResponse, nil
	}

	entry, ok := s.statusResponseCache[hash]
	if !ok {
		hash, newStatusResp, err := s.requestNewStatusResponseJSON(p)
		if err != nil {
			return status.ResponseJSON{}, err
		}

		s.statusResponseCache[hash] = &statusCacheEntry{
			expiresAt:      time.Now().Add(s.cacheTTL),
			statusResponse: newStatusResp,
		}

		return newStatusResp, nil
	}
	return entry.statusResponse, nil
}

func (s *statusResponseJSONProvider) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiredHashes := []uint64{}
	for hash, entry := range s.statusResponseCache {
		if entry.isExpired() {
			delete(s.statusResponseCache, hash)
			expiredHashes = append(expiredHashes, hash)
		}
	}

	for protVer, hash := range s.statusHash {
		for _, expiredHash := range expiredHashes {
			if hash == expiredHash {
				delete(s.statusHash, protVer)
			}
		}
	}
}

type StatusResponseJSONProvider interface {
	StatusResponseJSON(p *Player) (status.ResponseJSON, error)
}

type Server struct {
	id                         infrared.ServerID
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
	gatewayIDs                 []infrared.GatewayID
	statusResponseJSONProvider StatusResponseJSONProvider
}

func (s Server) Edition() infrared.Edition {
	return infrared.JavaEdition
}

func (s Server) ID() infrared.ServerID {
	return s.id
}

func (s Server) Domains() []string {
	return s.domains
}

func (s Server) GatewayIDs() []infrared.GatewayID {
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

func (s Server) NewConn(p infrared.Conn) (infrared.Conn, error) {
	player := p.(*Player)
	player.serverID = s.id
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(player); err != nil {
			return nil, err
		}
		return nil, err
	}

	if player.handshake.IsStatusRequest() {
		defer rc.Close()
		if err := s.handleStatusPing(player); err != nil {
			return nil, err
		}
		return nil, infrared.ErrClientStatusRequest
	}

	if err := s.prepareConns(player, rc); err != nil {
		rc.Close()
		return nil, err
	}

	// Sends the handshake and the request or login packet to the server
	if err := rc.WritePackets(player.readPks...); err != nil {
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
	if err := s.dialTimeoutDisconnector.DisconnectPlayer(p, infrared.ApplyTemplates(
		infrared.TimeMessageTemplates(),
		infrared.PlayerMessageTemplates(p),
		infrared.ServerMessageTemplate(s),
	)); err != nil {
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
