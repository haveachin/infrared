package infrared

import (
	"errors"
	"net"
	"sync"

	"github.com/gertd/wild"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

var ErrClientStatusRequest = errors.New("disconnect after status")

type Server interface {
	ID() string
	Domains() []string
	GatewayIDs() []string
	HandleConn(c net.Conn) (Conn, error)
	Edition() Edition
}

type ServerGatewayConfig struct {
	Gateways []Gateway
	Servers  []Server
	InChan   <-chan Player
	OutChan  chan<- ConnTunnel
	Logger   *zap.Logger
	EventBus event.Bus
}

type ServerGateway struct {
	ServerGatewayConfig

	mu         sync.Mutex
	reloadChan chan func()
	quitChan   chan bool
	gwIDSrvIDs map[string][]string
	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domain wildcard expressions
	srvExprs map[string][]string
}

func (sg *ServerGateway) init() {
	sg.indexIDs()
	sg.compileDomainExprs()
}

func (sg *ServerGateway) indexIDs() {
	sg.gwIDSrvIDs = map[string][]string{}
	sg.srvs = map[string]Server{}
	for _, srv := range sg.Servers {
		sg.srvs[srv.ID()] = srv

		gwIDs := srv.GatewayIDs()
		for _, gwID := range gwIDs {
			srvIDs, ok := sg.gwIDSrvIDs[gwID]
			if !ok {
				sg.gwIDSrvIDs[gwID] = []string{srv.ID()}
				continue
			}
			sg.gwIDSrvIDs[gwID] = append(srvIDs, srv.ID())
		}
	}
}

func (sg *ServerGateway) compileDomainExprs() {
	sg.srvExprs = map[string][]string{}
	for _, srv := range sg.Servers {
		exprs := make([]string, len(srv.Domains()))
		copy(exprs, srv.Domains())
		sg.srvExprs[srv.ID()] = exprs
	}
}

func (sg *ServerGateway) findServer(gatewayID, domain string) (Server, string) {
	for _, srvID := range sg.gwIDSrvIDs[gatewayID] {
		for _, srvExpr := range sg.srvExprs[srvID] {
			if wild.Match(srvExpr, domain, true) {
				return sg.srvs[srvID], srvExpr
			}
		}
	}

	return nil, ""
}

func (sg *ServerGateway) Start() {
	sg.mu.Lock()
	sg.reloadChan = make(chan func())
	sg.quitChan = make(chan bool)
	sg.mu.Unlock()
	sg.init()

	for {
		select {
		case player, ok := <-sg.InChan:
			if !ok {
				sg.Logger.Debug("server gateway quitting; incoming channel was closed")
				return
			}
			logger := sg.Logger.With(logProcessedConn(player)...)
			logger.Debug("looking up server address")

			srv, matchedDomain := sg.findServer(player.GatewayID(), player.RequestedAddr())
			if srv == nil {
				logger.Info("failed to find server; disconnecting client")
				_ = player.DisconnectServerNotFound()
				continue
			}
			player.SetMatchedAddr(matchedDomain)

			logger = logger.With(logServer(srv)...)
			logger.Debug("found server")

			replyChan := sg.EventBus.Request(PrePlayerJoinEvent{
				Player:        player,
				Server:        srv,
				MatchedDomain: matchedDomain,
			}, PrePlayerJoinEventTopic)

			if isEventCanceled(replyChan, logger) {
				player.Close()
				continue
			}

			sg.OutChan <- ConnTunnel{
				Conn:          player,
				Server:        srv,
				MatchedDomain: matchedDomain,
			}
		case reload := <-sg.reloadChan:
			reload()
			sg.init()
		case <-sg.quitChan:
			sg.Logger.Debug("server gateway quitting; received quit signal")
			return
		}
	}
}

func (sg *ServerGateway) Reload(cfg ServerGatewayConfig) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sg.reloadChan == nil {
		return
	}

	sg.reloadChan <- func() {
		sg.ServerGatewayConfig = cfg
	}
}

func (sg *ServerGateway) Close() error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sg.quitChan == nil {
		return errors.New("server gateway was not running")
	}

	sg.quitChan <- true
	return nil
}
