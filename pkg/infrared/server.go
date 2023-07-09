package infrared

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/IGLOU-EU/go-wildcard"
	"github.com/cespare/xxhash"
	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
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

func WithServerDomains(dd ...ServerDomain) ServerConfigFunc {
	return func(cfg *ServerConfig) {
		cfg.Domains = dd
	}
}

func WithServerAddress(addr ServerAddress) ServerConfigFunc {
	return func(cfg *ServerConfig) {
		cfg.Address = addr
	}
}

type ServerConfig struct {
	Domains []ServerDomain `mapstructure:"domains"`
	Address ServerAddress  `mapstructure:"address"`
}

type Server struct {
	cfg                        ServerConfig
	statusResponseJSONProvider StatusResponseJSONProvider
}

func NewServer(fns ...ServerConfigFunc) *Server {
	var cfg ServerConfig
	for _, fn := range fns {
		fn(&cfg)
	}

	srv := &Server{
		cfg: cfg,
	}

	srv.statusResponseJSONProvider = &statusResponseJSONProvider{
		server:              srv,
		cacheTTL:            10 * time.Second,
		statusHash:          make(map[protocol.Version]uint64),
		statusResponseCache: make(map[uint64]*statusCacheEntry),
	}

	return srv
}

func (s Server) Dial() (*conn, error) {
	c, err := net.Dial("tcp", string(s.cfg.Address))
	if err != nil {
		return nil, err
	}

	return newConn(c), nil
}

type ServerRequest struct {
	Domain          ServerDomain
	IsLogin         bool
	ProtocolVersion protocol.Version
	ReadPks         [2]protocol.Packet
	ResponseChan    chan<- ServerRequestResponse
}

type ServerRequestResponse struct {
	Conn         *conn
	ResponseJSON status.ResponseJSON
	Err          error
}

type serverGateway struct {
	Servers     []*Server
	responder   ServerRequestResponder
	requestChan <-chan ServerRequest

	servers map[ServerDomain]*Server
}

func (sg *serverGateway) init() error {
	if len(sg.Servers) == 0 {
		return errors.New("server gateway: no servers to route to")
	}

	sg.servers = make(map[ServerDomain]*Server)
	for _, srv := range sg.Servers {
		for _, d := range srv.cfg.Domains {
			dStr := string(d)
			dStr = strings.ToLower(dStr)
			dmn := ServerDomain(dStr)
			sg.servers[dmn] = srv
		}
	}

	if sg.responder == nil {
		sg.responder = DialServerRequestResponder{}
	}

	return nil
}

func (sg *serverGateway) findServer(domain ServerDomain) *Server {
	for d, srv := range sg.servers {
		if wildcard.Match(string(d), string(domain)) {
			return srv
		}
	}
	return nil
}

func (sg *serverGateway) listenAndServe() error {
	if err := sg.init(); err != nil {
		return err
	}

	for req := range sg.requestChan {
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

type DialServerRequestResponder struct{}

func (r DialServerRequestResponder) RespondeToServerRequest(req ServerRequest, srv *Server) {
	if req.IsLogin {
		rc, err := srv.Dial()
		if err != nil {
			req.ResponseChan <- ServerRequestResponse{
				Err: err,
			}
			return
		}

		req.ResponseChan <- ServerRequestResponse{
			Conn: rc,
		}
		return
	}

	respJSON, err := srv.statusResponseJSONProvider.StatusResponseJSON(req.ProtocolVersion, req.ReadPks)
	if err != nil {
		req.ResponseChan <- ServerRequestResponse{
			Err: err,
		}
		return
	}

	req.ResponseChan <- ServerRequestResponse{
		ResponseJSON: respJSON,
	}
}

type StatusResponseJSONProvider interface {
	StatusResponseJSON(protocol.Version, [2]protocol.Packet) (status.ResponseJSON, error)
}

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

func (s *statusResponseJSONProvider) requestNewStatusResponseJSON(readPks [2]protocol.Packet) (uint64, status.ResponseJSON, error) {
	rc, err := s.server.Dial()
	if err != nil {
		return 0, status.ResponseJSON{}, err
	}

	if err := rc.WritePackets(readPks[0], readPks[1]); err != nil {
		return 0, status.ResponseJSON{}, err
	}

	var pk protocol.Packet
	if err := rc.ReadPacket(&pk); err != nil {
		return 0, status.ResponseJSON{}, err
	}
	rc.Close()

	hash := xxhash.New()
	pk.WriteTo(hash)

	var respPk status.ClientBoundResponse
	if err := respPk.Unmarshal(pk); err != nil {
		return 0, status.ResponseJSON{}, err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return 0, status.ResponseJSON{}, err
	}

	return hash.Sum64(), respJSON, nil
}

func (s *statusResponseJSONProvider) StatusResponseJSON(protVer protocol.Version, readPks [2]protocol.Packet) (status.ResponseJSON, error) {
	if s.cacheTTL <= 0 {
		_, statusResp, err := s.requestNewStatusResponseJSON(readPks)
		return statusResp, err
	}

	// Prunes all expired status reponses
	s.prune()

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, ok := s.statusHash[protVer]
	if !ok {
		hash, newStatusResp, err := s.requestNewStatusResponseJSON(readPks)
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
		hash, newStatusResp, err := s.requestNewStatusResponseJSON(readPks)
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
