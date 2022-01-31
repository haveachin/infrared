package bedrock

import (
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/sandertv/go-raknet"
)

type Server struct {
	ID                 string
	Domains            []string
	Dialer             raknet.Dialer
	DialTimeout        time.Duration
	Address            string
	SendProxyProtocol  bool
	DialTimeoutMessage string
	WebhookIDs         []string
	Log                logr.Logger
}

func (s Server) GetSendProxyProtocol() bool {
	return s.SendProxyProtocol
}

func (s Server) GetID() string {
	return s.ID
}

func (s Server) GetDomains() []string {
	return s.Domains
}

func (s Server) GetWebhookIDs() []string {
	return s.WebhookIDs
}

func (s *Server) SetLogger(log logr.Logger) {
	s.Log = log
}

func (s Server) Dial() (*raknet.Conn, error) {
	c, err := s.Dialer.DialTimeout(s.Address, s.DialTimeout)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s Server) handleDialTimeout(c ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.DialTimeoutMessage, c, &s)
	return c.disconnect(msg)
}

func (s Server) ProcessConn(c net.Conn, webhooks []webhook.Webhook) (infrared.ConnTunnel, error) {
	pc := c.(*ProcessedConn)
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(*pc); err != nil {
			return infrared.ConnTunnel{}, err
		}
		return infrared.ConnTunnel{}, err
	}

	if _, err := rc.Write(pc.readBytes); err != nil {
		rc.Close()
		return infrared.ConnTunnel{}, err
	}

	return infrared.ConnTunnel{
		Conn:       pc,
		RemoteConn: rc,
	}, nil
}
