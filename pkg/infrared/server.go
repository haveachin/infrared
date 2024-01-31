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

type (
	ServerID      string
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

func (s Server) Dial() (*Conn, error) {
	c, err := net.Dial("tcp", string(s.cfg.Addresses[0]))
	if err != nil {
		return nil, err
	}

	conn := newConn(c)
	conn.timeout = time.Second * 10
	return conn, nil
}

type ServerRequest struct {
	Domain          ServerDomain
	IsLogin         bool
	ProtocolVersion protocol.Version
	ReadPks         [2]protocol.Packet
	ResponseChan    chan<- ServerRequestResponse
}

type ServerRequestResponse struct {
	ServerConn        *Conn
	StatusResponse    protocol.Packet
	SendProxyProtocol bool
	Err               error
}

type serverGateway struct {
	servers   []*Server
	responder ServerRequestResponder

	srvs map[ServerDomain]*Server
}

func (sg *serverGateway) init() error {
	if len(sg.servers) == 0 {
		return errors.New("server gateway: no servers to route to")
	}

	sg.srvs = make(map[ServerDomain]*Server)
	for _, srv := range sg.servers {
		for _, d := range srv.cfg.Domains {
			dStr := string(d)
			dStr = strings.ToLower(dStr)
			dmn := ServerDomain(dStr)
			sg.srvs[dmn] = srv
		}
	}

	if sg.responder == nil {
		sg.responder = DialServerRequestResponder{
			respProvs: make(map[*Server]StatusResponseProvider),
		}
	}

	return nil
}

func (sg *serverGateway) findServer(domain ServerDomain) *Server {
	dm := string(domain)
	dm = strings.ToLower(dm)
	for d, srv := range sg.srvs {
		if wildcard.Match(string(d), dm) {
			return srv
		}
	}
	return nil
}

func (sg *serverGateway) listenAndServe(reqChan <-chan ServerRequest) error {
	if err := sg.init(); err != nil {
		return err
	}

	for req := range reqChan {
		srv := sg.findServer(req.Domain)
		if srv == nil {
			req.ResponseChan <- ServerRequestResponse{
				Err: errors.New("server not found"),
			}
			continue
		}

		go sg.responder.RespondeToServerRequest(req, srv)
	}

	return nil
}

type ServerRequestResponder interface {
	RespondeToServerRequest(ServerRequest, *Server)
}

type DialServerRequestResponder struct {
	respProvs map[*Server]StatusResponseProvider
}

func (r DialServerRequestResponder) RespondeToServerRequest(req ServerRequest, srv *Server) {
	if req.IsLogin {
		rc, err := srv.Dial()

		req.ResponseChan <- ServerRequestResponse{
			ServerConn:        rc,
			Err:               err,
			SendProxyProtocol: srv.cfg.SendProxyProtocol,
		}
		return
	}

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

	_, pk, err := respProv.StatusResponse(req.ProtocolVersion, req.ReadPks)
	req.ResponseChan <- ServerRequestResponse{
		StatusResponse: pk,
		Err:            err,
	}
}

type StatusResponseProvider interface {
	StatusResponse(protocol.Version, [2]protocol.Packet) (status.ResponseJSON, protocol.Packet, error)
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
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return status.ResponseJSON{}, protocol.Packet{}, err
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
	protVer protocol.Version,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	if s.cacheTTL <= 0 {
		return s.requestNewStatusResponseJSON(readPks)
	}

	// Prunes all expired status reponses
	s.prune()

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, okHash := s.statusHash[protVer]
	entry, okCache := s.statusResponseCache[hash]
	if !okHash || !okCache {
		return s.cacheResponse(protVer, readPks)
	}

	return entry.responseJSON, entry.responsePk, nil
}

func (s *statusResponseProvider) cacheResponse(
	protVer protocol.Version,
	readPks [2]protocol.Packet,
) (status.ResponseJSON, protocol.Packet, error) {
	newStatusResp, pk, err := s.requestNewStatusResponseJSON(readPks)
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
