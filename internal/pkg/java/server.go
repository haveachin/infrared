package java

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pires/go-proxyproto"
	"golang.org/x/net/proxy"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type statusResponseJSONProvider struct {
	server          Server
	handshakePk     protocol.Packet
	statusRequestPk protocol.Packet

	mu                      sync.Mutex
	cacheTTL                time.Duration
	cacheSetAt              time.Time
	statusResponseJSONCache status.ResponseJSON
}

func (s *statusResponseJSONProvider) isStatusCacheValid() bool {
	return s.cacheSetAt.Add(s.cacheTTL).After(time.Now())
}

func (s *statusResponseJSONProvider) requestNewStatusResponseJSON() (status.ResponseJSON, error) {
	rc, err := s.server.dial()
	if err != nil {
		return status.ResponseJSON{}, err
	}

	if err := rc.WritePackets(s.handshakePk, s.statusRequestPk); err != nil {
		return status.ResponseJSON{}, err
	}

	pk, err := rc.ReadPacket(status.MaxSizeClientBoundResponse)
	if err != nil {
		return status.ResponseJSON{}, err
	}
	rc.Close()

	respPk, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return status.ResponseJSON{}, err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return status.ResponseJSON{}, err
	}

	return respJSON, nil
}

func (s *statusResponseJSONProvider) StatusResponseJSON() (status.ResponseJSON, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isStatusCacheValid() {
		return s.statusResponseJSONCache, nil
	}

	respJSON, err := s.requestNewStatusResponseJSON()
	if err != nil {
		return status.ResponseJSON{}, err
	}

	s.statusResponseJSONCache = respJSON
	s.cacheSetAt = time.Now()

	return respJSON, nil
}

type StatusResponseJSONProvider interface {
	StatusResponseJSON() (status.ResponseJSON, error)
}

type Server struct {
	ID                          string
	Domains                     []string
	Addr                        string
	AddrHost                    string
	AddrPort                    int
	SendProxyProtocol           bool
	SendRealIP                  bool
	OverrideAddress             bool
	Dialer                      proxy.Dialer
	OverrideStatus              OverrideStatusResponse
	OverrideStatusCacheDeadline time.Time
	DialTimeoutMessage          string
	DialTimeoutStatusJSON       string
	GatewayIDs                  []string

	statusResponseJSONProvider StatusResponseJSONProvider
}

func (s Server) dial() (*Conn, error) {
	c, err := s.Dialer.Dial("tcp", s.Addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}, nil
}

func (s Server) handleConn(c net.Conn) (infrared.Conn, error) {
	pc := c.(*ProcessedConn)
	if pc.handshake.IsStatusRequest() {
		if err := s.handleStatusPing(pc); err != nil {
			return nil, err
		}
		return nil, infrared.ErrClientStatusRequest
	}

	rc, err := s.dial()
	if err != nil {
		if err := s.handleDialTimeout(pc); err != nil {
			return nil, err
		}
		return nil, err
	}

	if s.SendProxyProtocol {
		if err := writeProxyProtocolHeader(pc.RemoteAddr(), rc); err != nil {
			defer rc.Close()
			return nil, err
		}
	}

	if s.SendRealIP {
		pc.handshake.UpgradeToRealIP(pc.RemoteAddr(), time.Now())
		pc.readPks[0] = pc.handshake.Marshal()
	}

	if s.OverrideAddress {
		pc.handshake.SetServerAddress(s.AddrHost)
		pc.handshake.ServerPort = protocol.UnsignedShort(s.AddrPort)
		pc.readPks[0] = pc.handshake.Marshal()
	}

	// Sends the handshake and the request or login packet to the server
	if err := rc.WritePackets(pc.readPks...); err != nil {
		defer rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s Server) handleDialTimeoutStatusRequest(pc *ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.DialTimeoutStatusJSON, pc, s.InfraredServer())
	respPk := status.ClientBoundResponse{
		JSONResponse: protocol.String(msg),
	}.Marshal()

	if err := pc.WritePacket(respPk); err != nil {
		return err
	}

	ping, err := pc.ReadPacket(status.MaxSizeServerBoundPingRequest)
	if err != nil {
		return err
	}

	return pc.WritePacket(ping)
}

func (s Server) handleDialTimeoutLoginRequest(pc *ProcessedConn) error {
	msg := infrared.ExecuteMessageTemplate(s.DialTimeoutMessage, pc)
	pk := login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
	}.Marshal()
	return pc.WritePacket(pk)
}

func (s Server) handleDialTimeout(c *ProcessedConn) error {
	if c.handshake.IsStatusRequest() {
		if err := s.handleDialTimeoutStatusRequest(c); err != nil {
			return err
		}
		return infrared.ErrClientStatusRequest
	}

	return s.handleDialTimeoutLoginRequest(c)
}

func (s Server) handleStatusPing(pc *ProcessedConn) error {
	if err := s.overrideStatusResponse(pc); err != nil {
		return err
	}

	ping, err := pc.ReadPacket(status.MaxSizeServerBoundPingRequest)
	if err != nil {
		return err
	}

	return pc.WritePacket(ping)
}

func (s Server) overrideStatusResponse(pc *ProcessedConn) error {
	respJSON, err := s.statusResponseJSONProvider.StatusResponseJSON()
	if err != nil {
		return err
	}

	respJSON = s.OverrideStatus.ResponseJSON(respJSON)
	respJSON.Description.Text = infrared.ExecuteServerMessageTemplate(respJSON.Description.Text, pc, s.InfraredServer())
	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	return pc.WritePacket(status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal())
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	tp := proxyproto.TCPv4
	tcpAddr := addr.(*net.TCPAddr)
	if tcpAddr.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        addr,
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}

func (s Server) InfraredServer() infrared.Server {
	return InfraredServer{
		Server: s,
	}
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

func (s InfraredServer) Dial() (infrared.Conn, error) {
	return s.Server.dial()
}
func (s InfraredServer) HandleConn(c net.Conn) (infrared.Conn, error) {
	return s.Server.handleConn(c)
}
