package infrared

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

var ErrClientStatusRequest = errors.New("disconnect after status")

type Server interface {
	ID() string
	Domains() []string
	WebhookIDs() []string
	ProcessConn(c net.Conn) (ConnTunnel, error)
	SetLogger(log logr.Logger)
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
	Log      logr.Logger

	gwIDSrvIDs map[string][]string
	// Gateway ID mapped to gateway
	gws map[string]Gateway
	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domains in lowercase
	srvDomains map[string][]string
}

func (sg *ServerGateway) indexServers() {
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

func (sg ServerGateway) findServer(gatewayID, domain string) Server {
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

func (sg ServerGateway) Start(srvChan <-chan ProcessedConn, poolChan chan<- ConnTunnel) {
	sg.indexServers()

	for {
		pc, ok := <-srvChan
		if !ok {
			break
		}

		keysAndValues := []interface{}{
			"network", pc.LocalAddr().Network(),
			"localAddr", pc.LocalAddr().String(),
			"remoteAddr", pc.RemoteAddr().String(),
			"serverAddr", pc.ServerAddr(),
			"username", pc.Username(),
			"gatewayId", pc.GatewayID(),
			"isLoginRequest", pc.IsLoginRequest(),
		}

		sg.Log.Info("looking up server address", keysAndValues...)

		srv := sg.findServer(pc.GatewayID(), pc.ServerAddr())
		if srv == nil {
			sg.Log.Info("failed to find server; disconnecting client", keysAndValues)
			_ = pc.DisconnectServerNotFound()
			continue
		}

		keysAndValues = append(keysAndValues,
			"serverId", srv.ID(),
			"serverDomains", srv.Domains(),
			"serverWebhookIds", srv.WebhookIDs(),
		)

		sg.Log.Info("starting proxy tunnel", keysAndValues...)
		event.Push(PreServerConnConnectingEventTopic, keysAndValues...)

		ct, err := srv.ProcessConn(pc)
		if err != nil {
			if errors.Is(err, ErrClientStatusRequest) {
				sg.Log.Info("disconnecting client; was status request", keysAndValues...)
			} else {
				sg.Log.Error(err, "failed to create proxy tunnel", keysAndValues...)
			}
			ct.Close()
			continue
		}

		keysAndValues = append(keysAndValues,
			"serverLocalAddr", ct.RemoteConn.LocalAddr().String(),
			"serverRemoteAddr", ct.RemoteConn.RemoteAddr().String(),
		)

		sg.Log.Info("adding proxy tunnel to pool", keysAndValues...)
		event.Push(ClientJoinEventTopic, keysAndValues...)

		ct.Metadata = keysAndValues
		poolChan <- ct
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
