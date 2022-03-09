package infrared

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

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

type ServerGateway struct {
	Gateways []Gateway
	Servers  []Server
	Log      *zap.Logger

	mu         sync.Mutex
	gwIDSrvIDs map[string][]string
	// Gateway ID mapped to gateway
	gws map[string]Gateway
	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domains in lowercase
	srvDomains map[string][]string
}

func (sg *ServerGateway) indexServers() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.gwIDSrvIDs = map[string][]string{}
	sg.gws = map[string]Gateway{}
	for _, gw := range sg.Gateways {
		sg.gwIDSrvIDs[gw.ID()] = gw.ServerIDs()
		sg.gws[gw.ID()] = gw
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

func (sg *ServerGateway) Start(srvChan <-chan ProcessedConn, poolChan chan<- ConnTunnel) {
	sg.indexServers()

	for pc := range srvChan {
		pcLogger := sg.Log.With(logProcessedConn(pc)...)
		pcLogger.Debug("looking up server address")

		srv := sg.findServer(pc.GatewayID(), pc.ServerAddr())
		if srv == nil {
			pcLogger.Debug("failed to find server; disconnecting client")
			_ = pc.DisconnectServerNotFound()
			continue
		}

		pcLogger = pcLogger.With(logServer(srv)...)
		pcLogger.Info("starting to proxy connection")
		event.Push(PreServerConnConnectingEventTopic, nil)

		poolChan <- ConnTunnel{
			Conn:   pc,
			Server: srv,
		}
	}
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
