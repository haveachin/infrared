package infrared

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/IGLOU-EU/go-wildcard"
	"github.com/cespare/xxhash/v2"
	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

var (
	ErrNoServers = errors.New("no servers to route to")
)

type (
	ServerAddress string
	ServerDomain  string
)

type ServerConfigFunc func(cfg *ServerConfig)

func WithServerConfig(c ServerConfig) ServerConfigFunc {
	return func(cfg *ServerConfig) {
		*cfg = c
	}
}

func WithServerDomains(sd ...ServerDomain) ServerConfigFunc {
	return func(cfg *ServerConfig) {
		cfg.Domains = sd
	}
}

func WithServerAddresses(addr ...ServerAddress) ServerConfigFunc {
	return func(cfg *ServerConfig) {
		cfg.Addresses = addr
	}
}

type ServerConfig struct {
	Domains           []ServerDomain  `yaml:"domains"`
	Addresses         []ServerAddress `yaml:"addresses"`
	SendProxyProtocol bool            `yaml:"sendProxyProtocol"`
}

type Server struct {
	cfg ServerConfig
}

func NewServer(fns ...ServerConfigFunc) (*Server, error) {
	var cfg ServerConfig
	for _, fn := range fns {
		fn(&cfg)
	}

	if len(cfg.Addresses) == 0 {
		return nil, errors.New("no addresses")
	}

	return &Server{
		cfg: cfg,
	}, nil
}

func (s Server) Dial() (*ServerConn, error) {
	c, err := net.Dial("tcp", string(s.cfg.Addresses[0]))
	if err != nil {
		return nil, err
	}

	return NewServerConn(c), nil
}

type ServerRequest struct {
	ClientAddr      net.Addr
	Domain          ServerDomain
	IsLogin         bool
	ProtocolVersion protocol.Version
	ReadPackets     [2]protocol.Packet
}

type ServerResponse struct {
	ServerConn        *ServerConn
	StatusResponse    protocol.Packet
	SendProxyProtocol bool
}

type ServerRequester interface {
	RequestServer(ServerRequest) (ServerResponse, error)
}

type ServerRequesterFunc func(ServerRequest) (ServerResponse, error)

func (fn ServerRequesterFunc) RequestServer(req ServerRequest) (ServerResponse, error) {
	return fn(req)
}

type ServerGateway struct {
	responder ServerRequestResponder
	servers   map[ServerDomain]*Server
}

func NewServerGateway(servers []*Server, responder ServerRequestResponder) (*ServerGateway, error) {
	if len(servers) == 0 {
		return nil, ErrNoServers
	}

	srvs := make(map[ServerDomain]*Server)
	for _, srv := range servers {
		for _, d := range srv.cfg.Domains {
			dStr := string(d)
			dStr = strings.ToLower(dStr)
			dmn := ServerDomain(dStr)
			srvs[dmn] = srv
		}
	}

	if responder == nil {
		responder = DialServerResponder{
			respProvs: make(map[*Server]StatusResponseProvider),
		}
	}

	return &ServerGateway{
		servers:   srvs,
		responder: responder,
	}, nil
}

func (sg *ServerGateway) findServer(domain ServerDomain) *Server {
	dm := string(domain)
	dm = strings.ToLower(dm)
	for d, srv := range sg.servers {
		if wildcard.Match(string(d), dm) {
			return srv
		}
	}
	return nil
}

func (sg *ServerGateway) RequestServer(req ServerRequest) (ServerResponse, error) {
	srv := sg.findServer(req.Domain)
	if srv == nil {
		return ServerResponse{}, errors.New("server not found")
	}

	return sg.responder.RespondeToServerRequest(req, srv)
}

type ServerRequestResponder interface {
	RespondeToServerRequest(ServerRequest, *Server) (ServerResponse, error)
}

type DialServerResponder struct {
	respProvs map[*Server]StatusResponseProvider
}

func (r DialServerResponder) RespondeToServerRequest(req ServerRequest, srv *Server) (ServerResponse, error) {
	if req.IsLogin {
		return r.respondeToLoginRequest(req, srv)
	}

	return r.respondeToStatusRequest(req, srv)
}

func (r DialServerResponder) respondeToLoginRequest(_ ServerRequest, srv *Server) (ServerResponse, error) {
	rc, err := srv.Dial()
	if err != nil {
		return ServerResponse{}, err
	}

	return ServerResponse{
		ServerConn:        rc,
		SendProxyProtocol: srv.cfg.SendProxyProtocol,
	}, nil
}

func (r DialServerResponder) respondeToStatusRequest(req ServerRequest, srv *Server) (ServerResponse, error) {
	respProv, ok := r.respProvs[srv]
	if !ok {
		respProv = &statusResponseProvider{
			server:              srv,
			cacheTTL:            30 * time.Second,
			statusHash:          make(map[protocol.Version]uint64),
			statusResponseCache: make(map[uint64]*statusCacheEntry),
		}
		r.respProvs[srv] = respProv
	}

	_, pk, err := respProv.StatusResponse(req.ClientAddr, req.ProtocolVersion, req.ReadPackets)
	if err != nil {
		return ServerResponse{}, err
	}

	return ServerResponse{
		StatusResponse: pk,
	}, nil
}

type StatusResponseProvider interface {
	StatusResponse(net.Addr, protocol.Version, [2]protocol.Packet) (status.ResponseJSON, protocol.Packet, error)
}

type statusCacheEntry struct {
	expiresAt    time.Time
	responseJSON status.ResponseJSON
	responsePk   protocol.Packet
}

func (e statusCacheEntry) isExpired() bool {
	return e.expiresAt.Before(time.Now())
}

type statusResponseProvider struct {
	server *Server

	mu                  sync.Mutex
	cacheTTL            time.Duration
	statusHash          map[protocol.Version]uint64
	statusResponseCache map[uint64]*statusCacheEntry
}

func (s *statusResponseProvider) requestNewStatusResponseJSON(
	cliAddr net.Addr,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}

	if s.server.cfg.SendProxyProtocol {
		if err := writeProxyProtocolHeader(cliAddr, rc); err != nil {
			return status.ResponseJSON{}, protocol.Packet{}, err
		}
	}

	if err := rc.WritePackets(readPks[0], readPks[1]); err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}

	var pk protocol.Packet
	if err := rc.ReadPacket(&pk); err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}
	rc.Close()

	var respPk status.ClientBoundResponse
	if err := respPk.Unmarshal(pk); err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}

	return respJSON, pk, nil
}

func (s *statusResponseProvider) StatusResponse(
	cliAddr net.Addr,
	protVer protocol.Version,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	if s.cacheTTL <= 0 {
		return s.requestNewStatusResponseJSON(cliAddr, readPks)
	}

	// Prunes all expired status reponses
	s.prune()

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, okHash := s.statusHash[protVer]
	entry, okCache := s.statusResponseCache[hash]
	if !okHash || !okCache {
		return s.cacheResponse(cliAddr, protVer, readPks)
	}

	return entry.responseJSON, entry.responsePk, nil
}

func (s *statusResponseProvider) cacheResponse(
	cliAddr net.Addr,
	protVer protocol.Version,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	newStatusResp, pk, err := s.requestNewStatusResponseJSON(cliAddr, readPks)
	if err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
	}

	hash := xxhash.New().Sum64()
	s.statusHash[protVer] = hash

	s.statusResponseCache[hash] = &statusCacheEntry{
		expiresAt:    time.Now().Add(s.cacheTTL),
		responseJSON: newStatusResp,
		responsePk:   pk,
	}

	return newStatusResp, pk, nil
}

func (s *statusResponseProvider) prune() {
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
