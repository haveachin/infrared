package java

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/pires/go-proxyproto"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
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
}

type InfraredServer struct {
	Server
}

func (s InfraredServer) ID() string {
	return s.Server.ID
}

func (s InfraredServer) Domains() []string {
	return s.Server.Domains
}

func (s InfraredServer) WebhookIDs() []string {
	return s.Server.WebhookIDs
}

func (s InfraredServer) Dial() (*Conn, error) {
	c, err := s.Dialer.Dial("tcp", s.Address)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}, nil
}

func (s InfraredServer) ProcessConn(c net.Conn) (net.Conn, error) {
	pc := c.(*ProcessedConn)
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(*pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if s.SendProxyProtocol {
		if err := writeProxyProtocolHeader(pc, rc); err != nil {
			rc.Close()
			return nil, err
		}
	}

	if s.SendRealIP {
		pc.handshake.UpgradeToRealIP(pc.RemoteAddr(), time.Now())
		pc.readPks[0] = pc.handshake.Marshal()
	}

	// TODO: Cache server status response
	if err := rc.WritePackets(pc.readPks...); err != nil {
		rc.Close()
		return nil, err
	}

	if pc.handshake.IsStatusRequest() {
		defer rc.Close()
		if err := s.handleStatusPing(*pc, *rc); err != nil {
			return nil, err
		}
		return nil, infrared.ErrClientStatusRequest
	}

	return rc, nil
}

func (s InfraredServer) handleDialTimeoutStatusRequest(c ProcessedConn) error {
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

func (s InfraredServer) handleDialTimeoutLoginRequest(pc ProcessedConn) error {
	msg := infrared.ExecuteMessageTemplate(s.DialTimeoutMessage, &pc)
	pk := login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
	}.Marshal()
	return pc.WritePacket(pk)
}

func (s InfraredServer) handleDialTimeout(c ProcessedConn) error {
	if c.handshake.IsStatusRequest() {
		if err := s.handleDialTimeoutStatusRequest(c); err != nil {
			return err
		}
		return infrared.ErrClientStatusRequest
	}

	return s.handleDialTimeoutLoginRequest(c)
}

func (s InfraredServer) handleStatusPing(pc ProcessedConn, rc Conn) error {
	if err := s.overrideStatusResponse(pc, rc); err != nil {
		return err
	}

	return pc.WritePacket(pc.readPks[1])
}

func (s InfraredServer) overrideStatusResponse(pc ProcessedConn, rc Conn) error {
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

	if err := pc.WritePacket(respPk.Marshal()); err != nil {
		return err
	}

	return nil
}

func writeProxyProtocolHeader(c, rc net.Conn) error {
	tp := proxyproto.TCPv4
	addr := c.RemoteAddr().(*net.TCPAddr)
	if addr.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        c.RemoteAddr(),
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}
