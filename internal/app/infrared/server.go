package infrared

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/webhook"
)

type Server interface {
	GetID() string
	GetDomains() []string
	GetWebhookIDs() []string
	ProcessConn(c net.Conn, webhooks []webhook.Webhook) (ConnTunnel, error)
	SetLogger(log logr.Logger)
}

func ExecuteMessageTemplate(msg string, pc ProcessedConn, s Server) string {
	tmpls := map[string]string{
		"username":      pc.Username(),
		"currentTime":   time.Now().Format(time.RFC822),
		"remoteAddress": pc.RemoteAddr().String(),
		"localAddress":  pc.LocalAddr().String(),
		"serverDomain":  pc.ServerAddr(),
		"serverID":      s.GetID(),
		"gatewayID":     pc.GatewayID(),
	}

	for k, v := range tmpls {
		msg = strings.Replace(msg, fmt.Sprintf("{{%s}}", k), v, -1)
	}

	return msg
}

type ServerGateway struct {
	GatewayIDServerIDs map[string][]string
	// ServerNotFoundMessages maps the GatewayID to server not found message
	ServerNotFoundMessages map[string]string
	Servers                []Server
	Webhooks               []webhook.Webhook
	Log                    logr.Logger

	// Server ID mapped to server
	srvs map[string]Server
	// Server ID mapped to server domains in lowercase
	srvDomains map[string][]string
	// Server ID mapped to webhooks
	srvWhks map[string][]webhook.Webhook
}

func (sg *ServerGateway) indexServers() {
	sg.srvs = map[string]Server{}
	sg.srvDomains = map[string][]string{}
	for _, srv := range sg.Servers {
		sg.srvs[srv.GetID()] = srv

		dd := make([]string, len(srv.GetDomains()))
		for i, d := range srv.GetDomains() {
			dd[i] = strings.ToLower(d)
		}
		sg.srvDomains[srv.GetID()] = dd
	}
}

// indexWebhooks indexes the webhooks that servers use.
func (sg *ServerGateway) indexWebhooks() error {
	whks := map[string]webhook.Webhook{}
	for _, w := range sg.Webhooks {
		whks[w.ID] = w
	}

	sg.srvWhks = map[string][]webhook.Webhook{}
	for _, srv := range sg.Servers {
		ww := make([]webhook.Webhook, len(srv.GetWebhookIDs()))
		for n, id := range srv.GetWebhookIDs() {
			w, ok := whks[id]
			if !ok {
				return fmt.Errorf("webhook with ID %q doesn't exist", id)
			}
			ww[n] = w
		}
		sg.srvWhks[srv.GetID()] = ww
	}
	return nil
}

func (sg ServerGateway) findServer(gatewayID, domain string) Server {
	domain = strings.ToLower(domain)
	srvIDs := sg.GatewayIDServerIDs[gatewayID]

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

func (sg ServerGateway) Start(srvChan <-chan ProcessedConn, poolChan chan<- ConnTunnel) error {
	sg.indexServers()
	if err := sg.indexWebhooks(); err != nil {
		return err
	}

	for {
		pc, ok := <-srvChan
		if !ok {
			break
		}

		srv := sg.findServer(pc.GatewayID(), pc.ServerAddr())
		if srv == nil {
			sg.Log.Info("invalid server",
				"serverAddress", pc.ServerAddr(),
				"remoteAddress", pc.RemoteAddr(),
			)
			msg := sg.ServerNotFoundMessages[pc.GatewayID()]
			msg = ExecuteMessageTemplate(msg, pc, srv)
			_ = pc.Disconnect(msg)
			continue
		}

		sg.Log.Info("connecting client",
			"serverId", srv.GetID(),
			"remoteAddress", pc.RemoteAddr(),
		)

		whks := sg.srvWhks[srv.GetID()]
		ct, err := srv.ProcessConn(pc, whks)
		if err != nil {
			ct.Close()
			continue
		}

		// Shallow copy webhooks to mitigate race conditions
		whksCopy := make([]webhook.Webhook, len(whks))
		_ = copy(whksCopy, whks)
		ct.Webhooks = whksCopy

		poolChan <- ct
	}

	return nil
}

// wildcardSimilarity determines the similarity of a domain to a wildcard domain
// If the similarity ends on a '*' then the domain is compareable to the wildcard domain
// then the it returns the length of the equal string slice.
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
	return i
}
