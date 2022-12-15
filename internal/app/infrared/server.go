package infrared

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

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

func ExecuteServerMessageTemplate(msg string, pc ProcessedConn, s Server) string {
	tmpls := map[string]string{
		"serverId": s.ID(),
	}

	for k, v := range tmpls {
		msg = strings.Replace(msg, fmt.Sprintf("{{%s}}", k), v, -1)
	}

	return ExecuteMessageTemplate(msg, pc)
}

type ServerGatewayConfig struct {
	Gateways []Gateway
	Servers  []Server
	In       <-chan ProcessedConn
	Out      chan<- ConnTunnel
	Logger   *zap.Logger
	EventBus event.Bus
}

type ServerGateway struct {
	ServerGatewayConfig

	reload     chan func()
	quit       chan bool
	gwIDSrvIDs map[string][]string
	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domain regexps
	srvRegexps map[string][]*regexp.Regexp
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
	sg.srvRegexps = map[string][]*regexp.Regexp{}
	for _, srv := range sg.Servers {
		regexps := make([]*regexp.Regexp, 0, len(srv.Domains()))
		for _, expr := range srv.Domains() {
			regexp, err := regexp.Compile(expr)
			if err != nil {
				sg.Logger.Error("failed to compile expression",
					zap.Error(err),
					zap.String("expression", expr),
				)
				continue
			}
			regexps = append(regexps, regexp)
		}
		sg.srvRegexps[srv.ID()] = regexps
	}
}

func (sg *ServerGateway) findServer(gatewayID, domain string) (Server, string) {
	for _, srvID := range sg.gwIDSrvIDs[gatewayID] {
		for _, srvRegexp := range sg.srvRegexps[srvID] {
			if srvRegexp.MatchString(domain) {
				return sg.srvs[srvID], srvRegexp.String()
			}
		}
	}

	return nil, ""
}

func (sg *ServerGateway) Start() {
	sg.reload = make(chan func())
	sg.quit = make(chan bool)
	sg.init()

	for {
		select {
		case pc, ok := <-sg.In:
			if !ok {
				sg.Logger.Debug("server gateway quitting; incoming channel was closed")
				return
			}
			pcLogger := sg.Logger.With(logProcessedConn(pc)...)
			pcLogger.Debug("looking up server address")

			srv, matchedDomain := sg.findServer(pc.GatewayID(), pc.ServerAddr())
			if srv == nil {
				pcLogger.Info("failed to find server; disconnecting client")
				_ = pc.DisconnectServerNotFound()
				continue
			}

			pcLogger = pcLogger.With(logServer(srv)...)
			pcLogger.Debug("found server")

			replyChan := sg.EventBus.Request(PreConnConnectingEvent{
				ProcessedConn: pc,
				Server:        srv,
			}, PrePlayerJoinEventTopic)

			if isEventCanceled(replyChan, pcLogger) {
				pc.Close()
				continue
			}

			sg.Out <- ConnTunnel{
				Conn:          pc,
				Server:        srv,
				MatchedDomain: matchedDomain,
			}
		case reload := <-sg.reload:
			reload()
			sg.init()
		case <-sg.quit:
			sg.Logger.Debug("server gateway quitting; received quit signal")
			return
		}
	}
}

func (sg *ServerGateway) Reload(cfg ServerGatewayConfig) {
	sg.reload <- func() {
		sg.ServerGatewayConfig = cfg
	}
}

func (sg *ServerGateway) Close() error {
	if sg.quit == nil {
		return errors.New("server gateway was not running")
	}
	sg.quit <- true
	return nil
}
