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
	"github.com/rs/zerolog/log"
)

var (
	ErrNoServers          = errors.New("no servers to route to")
	ErrServerNotFound     = errors.New("server not found")
	ErrServerNotReachable = errors.New("server not reachable")
)

type (
	ServerAddress string
	ServerDomain  string
)

type ServerConfig struct {
	Domains                []ServerDomain               `yaml:"domains"`
	Addresses              []ServerAddress              `yaml:"addresses"`
	SendProxyProtocol      bool                         `yaml:"sendProxyProtocol"`
	DialTimeoutResponse    HandshakeResponseConfig      `yaml:"dialTimeoutResponse"`
	OverrideStatusResponse OverrideStatusResponseConfig `yaml:"overrideStatus"`
}

func NewServerConfig() ServerConfig {
	return ServerConfig{}
}

func (cfg ServerConfig) WithDomains(sd ...ServerDomain) ServerConfig {
	cfg.Domains = sd
	return cfg
}

func (cfg ServerConfig) WithAddresses(addr ...ServerAddress) ServerConfig {
	cfg.Addresses = addr
	return cfg
}

type Server struct {
	cfg ServerConfig

	dialTimeoutResp    HandshakeResponse
	overrideStatusResp OverrideStatusResponse
}

func NewServer(cfg ServerConfig) (*Server, error) {
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("no addresses")
	}

	return &Server{
		cfg: cfg,
		dialTimeoutResp: HandshakeResponse{
			Config: cfg.DialTimeoutResponse,
		},
		overrideStatusResp: OverrideStatusResponse{
			Config: cfg.OverrideStatusResponse,
		},
	}, nil
}

func (s *Server) Dial() (*ServerConn, error) {
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
	Packet            protocol.Packet
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

func (sg ServerGateway) findServer(domain ServerDomain) *Server {
	dm := string(domain)
	dm = strings.ToLower(dm)
	for d, srv := range sg.servers {
		if wildcard.Match(string(d), dm) {
			return srv
		}
	}
	return nil
}

func (sg ServerGateway) RequestServer(req ServerRequest) (ServerResponse, error) {
	srv := sg.findServer(req.Domain)
	if srv == nil {
		return ServerResponse{}, ErrServerNotFound
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
		return ServerResponse{
			Packet: srv.dialTimeoutResp.LoginReponse(),
		}, ErrServerNotReachable
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
			server: srv,
			cache: statusResponseCache{
				ttl:                 3 * time.Second,
				statusHash:          make(map[protocol.Version]uint64),
				statusResponseCache: make(map[uint64]*statusCacheEntry),
			},
		}
		r.respProvs[srv] = respProv
	}

	pk, err := respProv.StatusResponse(req.ClientAddr, req.ProtocolVersion, req.ReadPackets)
	if err != nil {
		return ServerResponse{}, err
	}

	return ServerResponse{
		Packet: pk,
	}, nil
}

type StatusResponseProvider interface {
	StatusResponse(net.Addr, protocol.Version, [2]protocol.Packet) (protocol.Packet, error)
}

type statusCacheEntry struct {
	expiresAt    time.Time
	responsePk   protocol.Packet
}

func (e statusCacheEntry) isExpired() bool {
	return e.expiresAt.Before(time.Now())
}

type statusResponseProvider struct {
	server *Server
	cache  statusResponseCache
}

func (s *statusResponseProvider) StatusResponse(
	cliAddr net.Addr,
	protVer protocol.Version,
	readPks [2]protocol.Packet,
) (protocol.Packet, error) {
	cacheRespone := false
	if s.cache.ttl > 0 {
	    statusPk, err := s.cache.statusResponse(protVer)
		if err == nil {
			return statusPk, nil
		}
		cacheRespone = true
	}

	statusResp, statusPk, err := s.requestNewStatusResponseJSON(cliAddr, readPks)
	if err != nil {
		switch {
		case errors.Is(err, ErrServerNotReachable):
			respPk := s.server.dialTimeoutResp.StatusResponse(protVer)
			return respPk, nil
		default:
			return protocol.Packet{}, err
		}
	}

	if cacheRespone {
		if err := s.cache.cacheStatusResponse(protVer, statusResp, statusPk); err != nil {
			return protocol.Packet{}, err
		}
	}

	return statusPk, nil
}

func (s *statusResponseProvider) requestNewStatusResponseJSON(
	cliAddr net.Addr,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, ErrServerNotReachable
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

type statusResponseCache struct {
	mu                  sync.Mutex
	ttl                 time.Duration
	statusHash          map[protocol.Version]uint64
	statusResponseCache map[uint64]*statusCacheEntry
}

func (s *statusResponseCache) statusResponse(
	protVer protocol.Version,
) (protocol.Packet, error) {
	// Prunes all expired status reponses
	s.prune()

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, okHash := s.statusHash[protVer]
	entry, okCache := s.statusResponseCache[hash]
	if !okHash || !okCache {
		return protocol.Packet{}, errors.New("not in cache")
	}

	return entry.responsePk, nil
}

func (s *statusResponseCache) cacheStatusResponse(
	protVer protocol.Version,
	statusResp status.ResponseJSON,
	respPk protocol.Packet,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := xxhash.New()
	hash.Write(statusResp.Description)
	hash.WriteString(statusResp.Favicon)
	hashSum := hash.Sum64()

	s.statusHash[protVer] = hashSum

	if _, ok := s.statusResponseCache[hashSum]; ok {
		log.Debug().
			Int32("protVer", int32(protVer)).
			Msg("Linked existing cache entry")

		return nil
	}

	s.statusResponseCache[hashSum] = &statusCacheEntry{
		expiresAt:    time.Now().Add(s.ttl),
		responsePk:   respPk,
	}

	log.Debug().
		Int32("protVer", int32(protVer)).
		Msg("Created new cache entry")

	return nil
}

func (s *statusResponseCache) prune() {
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

				log.Debug().
					Int32("protVer", int32(protVer)).
					Msg("Deleted cache entry")
			}
		}
	}
}
