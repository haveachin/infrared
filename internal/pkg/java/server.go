package java

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/pires/go-proxyproto"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
	"github.com/haveachin/infrared/pkg/webhook"
)

type Server struct {
	ID                 string
	Domains            []string
	Dialer             net.Dialer
	Address            string
	SendProxyProtocol  bool
	SendRealIP         bool
	DialTimeoutMessage string
	OverrideStatus     OverrideStatusResponse
	DialTimeoutStatus  DialTimeoutStatusResponse
	WebhookIDs         []string
	Log                logr.Logger
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

func (s Server) Dial() (Conn, error) {
	c, err := s.Dialer.Dial("tcp", s.Address)
	if err != nil {
		return Conn{}, err
	}

	return Conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}, nil
}

func (s Server) handleDialTimeoutStatusRequest(c ProcessedConn) error {
	respJSON, err := s.DialTimeoutStatus.ResponseJSON()
	if err != nil {
		return err
	}

	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	msg := infrared.ExecuteServerMessageTemplate(string(bb), &c, &s)
	respPk := status.ClientBoundResponse{
		JSONResponse: protocol.String(msg),
	}.Marshal()

	if err := c.WritePacket(respPk); err != nil {
		return err
	}

	pingPk, err := c.ReadPacket()
	if err != nil {
		return err
	}

	return c.WritePacket(pingPk)
}

func (s Server) handleDialTimeoutLoginRequest(pc ProcessedConn) error {
	msg := infrared.ExecuteMessageTemplate(s.DialTimeoutMessage, &pc)
	pk := login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
	}.Marshal()
	return pc.WritePacket(pk)
}

func (s Server) handleDialTimeout(c ProcessedConn) error {
	if c.handshake.IsStatusRequest() {
		return s.handleDialTimeoutStatusRequest(c)
	}

	return s.handleDialTimeoutLoginRequest(c)
}

func (s Server) overrideStatusResponse(c ProcessedConn, rc Conn) error {
	pk, err := rc.ReadPacket()
	if err != nil {
		return err
	}

	respPk, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return err
	}

	respJSON, err = s.OverrideStatus.ResponseJSON(respJSON)
	if err != nil {
		return err
	}

	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	respPk.JSONResponse = protocol.String(bb)

	if err := c.WritePacket(respPk.Marshal()); err != nil {
		return err
	}

	return nil
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

	if s.SendProxyProtocol {
		tp := proxyproto.TCPv4
		addr := pc.RemoteAddr().(*net.TCPAddr)
		if addr.IP.To4() == nil {
			tp = proxyproto.TCPv6
		}

		header := &proxyproto.Header{
			Version:           2,
			Command:           proxyproto.PROXY,
			TransportProtocol: tp,
			SourceAddr:        pc.RemoteAddr(),
			DestinationAddr:   rc.RemoteAddr(),
		}

		if _, err := header.WriteTo(&rc); err != nil {
			return infrared.ConnTunnel{}, err
		}
	}

	if s.SendRealIP {
		pc.handshake.UpgradeToRealIP(pc.RemoteAddr(), time.Now())
		pc.readPks[0] = pc.handshake.Marshal()
	}

	if err := rc.WritePackets(pc.readPks...); err != nil {
		rc.Close()
		return infrared.ConnTunnel{}, err
	}

	if pc.handshake.IsStatusRequest() {
		if err := s.overrideStatusResponse(*pc, rc); err != nil {
			rc.Close()
			return infrared.ConnTunnel{}, err
		}
	}

	return infrared.ConnTunnel{
		Conn:       pc,
		RemoteConn: rc.Conn,
	}, nil
}
