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
	WebhookIDs() []string
	HandleConn(c net.Conn) (net.Conn, error)
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

func (sg *ServerGateway) indexServers() {
	sg.gwIDSrvIDs = map[string][]string{}
	for _, gw := range sg.Gateways {
		sg.gwIDSrvIDs[gw.ID()] = gw.ServerIDs()
	}

	sg.srvs = map[string]Server{}
	sg.srvDomains = map[string][]string{}
	for _, srv := range sg.Servers {
		sg.srvs[srv.ID()] = srv

		dd := make([]string, len(srv.Domains()))
		for i, d := range srv.Domains() {
			dd[i] = strings.ToLower(d)
		}
		sg.srvDomains[srv.ID()] = dd
	}
}

func (sg *ServerGateway) findServer(gatewayID, domain string) Server {
	domain = strings.ToLower(domain)
	srvIDs := sg.gwIDSrvIDs[gatewayID]

	var hs int
	var srv Server
	for _, id := range srvIDs {
		for _, d := range sg.srvDomains[id] {
			cs := wildcardSimilarity(domain, d)
			if cs > -1 && cs >= hs {
				hs = cs
				srv = sg.srvs[id]
			}
		}
	}
	return srv
}

func (sg *ServerGateway) Start() {
	sg.reload = make(chan func())
	sg.quit = make(chan bool)
	sg.indexServers()

	for {
		select {
		case pc, ok := <-sg.In:
			if !ok {
				sg.Logger.Debug("server gateway quitting; incoming channel was closed")
				return
			}
			pcLogger := sg.Logger.With(logProcessedConn(pc)...)
			pcLogger.Debug("looking up server address")

			srv := sg.findServer(pc.GatewayID(), pc.ServerAddr())
			if srv == nil {
				pcLogger.Debug("failed to find server; disconnecting client")
				_ = pc.DisconnectServerNotFound()
				continue
			}

			pcLogger = pcLogger.With(logServer(srv)...)
			pcLogger.Info("starting to proxy connection")
			event.Push(PreConnConnectingEventTopic, PreConnConnectingEvent{
				ProcessedConn: pc,
				Server:        srv,
			})

			sg.Out <- ConnTunnel{
				Conn:   pc,
				Server: srv,
			}
		case reload := <-sg.reload:
			reload()
			sg.indexServers()
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
// If the similarity ends on a '*' then the domain is compareable to the wildcard domain
// then it returns the length of the equal string slice. If it is an exact match
// then it returns the length of the domain string + 1.
// Else if they are not compareable because the equal string slice ends on any rune
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
			// If the similarity does not end on a wildcard then return -1 for no compareable
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
