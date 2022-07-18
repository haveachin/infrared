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
	ID                          string
	Domains                     []string
	Addr                        string
	SendProxyProtocol           bool
	SendRealIP                  bool
	OverrideAddress             bool
	Dialer                      net.Dialer
	OverrideStatus              OverrideStatusResponse
	OverrideStatusCacheDeadline time.Time
	DialTimeoutMessage          string
	DialTimeoutStatusJSON       string
	GatewayIDs                  []string

	Host string
	Port int

	overrideStatusCache *string
}

type InfraredServer struct {
	Server Server
}

func (s InfraredServer) Edition() infrared.Edition {
	return infrared.JavaEdition
}

func (s InfraredServer) ID() string {
	return s.Server.ID
}

func (s InfraredServer) Domains() []string {
	return s.Server.Domains
}

func (s InfraredServer) GatewayIDs() []string {
	return s.Server.GatewayIDs
}

func (s InfraredServer) Dial() (*Conn, error) {
	c, err := s.Server.Dialer.Dial("tcp", s.Server.Addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}, nil
}

func (s InfraredServer) HandleConn(c net.Conn) (infrared.Conn, error) {
	pc := c.(*ProcessedConn)
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if s.Server.SendProxyProtocol {
		if err := writeProxyProtocolHeader(pc, rc); err != nil {
			defer rc.Close()
			return nil, err
		}
	}

	if s.Server.SendRealIP {
		pc.handshake.UpgradeToRealIP(pc.RemoteAddr(), time.Now())
		pc.readPks[0] = pc.handshake.Marshal()
	}

	if s.Server.OverrideAddress {
		pc.handshake.ServerAddress = protocol.String(s.Server.Host)
		pc.handshake.ServerPort = protocol.UnsignedShort(s.Server.Port)
		pc.readPks[0] = pc.handshake.Marshal()
	}

	// Sends the handshake and the request or login packet to the server
	if err := rc.WritePackets(pc.readPks...); err != nil {
		defer rc.Close()
		return nil, err
	}

	if pc.handshake.IsStatusRequest() {
		defer rc.Close()
		if err := s.handleStatusPing(pc, rc); err != nil {
			return nil, err
		}
		return nil, infrared.ErrClientStatusRequest
	}

	return rc, nil
}

func (s InfraredServer) handleDialTimeoutStatusRequest(pc *ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.Server.DialTimeoutStatusJSON, pc, &s)
	respPk := status.ClientBoundResponse{
		JSONResponse: protocol.String(msg),
	}.Marshal()

	if err := pc.WritePacket(respPk); err != nil {
		return err
	}

	ping, err := pc.ReadPacket()
	if err != nil {
		return err
	}

	return pc.WritePacket(ping)
}

func (s InfraredServer) handleDialTimeoutLoginRequest(pc *ProcessedConn) error {
	msg := infrared.ExecuteMessageTemplate(s.Server.DialTimeoutMessage, pc)
	pk := login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
	}.Marshal()
	return pc.WritePacket(pk)
}

func (s InfraredServer) handleDialTimeout(c *ProcessedConn) error {
	if c.handshake.IsStatusRequest() {
		if err := s.handleDialTimeoutStatusRequest(c); err != nil {
			return err
		}
		return infrared.ErrClientStatusRequest
	}

	return s.handleDialTimeoutLoginRequest(c)
}

func (s InfraredServer) handleStatusPing(pc *ProcessedConn, rc *Conn) error {
	if err := s.overrideStatusResponse(pc, rc); err != nil {
		return err
	}

	ping, err := pc.ReadPacket()
	if err != nil {
		return err
	}

	return pc.WritePacket(ping)
}

func (s InfraredServer) overrideStatusResponse(pc *ProcessedConn, rc *Conn) error {
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

	respJSON = s.Server.OverrideStatus.ResponseJSON(respJSON)
	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	respPk.JSONResponse = protocol.String(bb)
	return pc.WritePacket(respPk.Marshal())
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
