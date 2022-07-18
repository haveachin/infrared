package infrared

import (
	"errors"
	"fmt"
	"net"
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
}

type ServerGateway struct {
	ServerGatewayConfig

	reload     chan func()
	quit       chan bool
	gwIDSrvIDs map[string][]string
	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domains in lowercase
	srvDomains map[string][]string
}

func (sg *ServerGateway) init() {
	sg.indexIDs()
	sg.indexDomains()
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

func (sg *ServerGateway) indexDomains() {
	sg.srvDomains = map[string][]string{}
	for _, srv := range sg.Servers {
		dd := make([]string, len(srv.Domains()))
		for i, d := range srv.Domains() {
			dd[i] = strings.ToLower(d)
		}
		sg.srvDomains[srv.ID()] = dd
	}
}

func (sg *ServerGateway) findServer(gatewayID, domain string) (Server, string) {
	domain = strings.ToLower(domain)
	srvIDs := sg.gwIDSrvIDs[gatewayID]

	var hs int
	var srv Server
	var matchedDomain string
	for _, srvID := range srvIDs {
		for _, srvDomain := range sg.srvDomains[srvID] {
			srvDomain = strings.ToLower(srvDomain)
			cs := wildcardSimilarity(domain, srvDomain)
			if cs > -1 && cs >= hs {
				hs = cs
				srv = sg.srvs[srvID]
				matchedDomain = srvDomain
			}
		}
	}
	return srv, matchedDomain
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
			event.Push(PreConnConnectingEvent{
				ProcessedConn: pc,
				Server:        srv,
			}, PreConnConnectingEventTopic)

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

// wildcardSimilarity determines the similarity of a domain to a wildcard domain
// If the similarity ends on a '*' then the domain is comparable to the wildcard domain
// then it returns the length of the equal string slice. If it is an exact match
// then it returns the length of the domain string + 1.
// Else if they are not comparable because the equal string slice ends on any rune
// that is not '*' it returns -1
func wildcardSimilarity(domain, wildcardDomain string) int {
	ra, rb := []rune(domain), []rune(wildcardDomain)
	la, lb := len(domain)-1, len(wildcardDomain)-1

	// Determine shorter string length
	var sl int
	if la > lb {
		sl = lb
	} else {
		sl = la
	}

	i := 0
	for i = 0; i <= sl; i++ {
		if ra[la-i] != rb[lb-i] {
			// If the similarity does not end on a wildcard then return -1 for no comparable
			if rb[lb-i] != '*' {
				return -1
			}
			break
		}
	}

	// If it is an exact match then make it the most similar
	if i == lb {
		i++
	}

	return i
}
