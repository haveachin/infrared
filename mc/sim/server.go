package sim

import (
	"fmt"
	"strings"

	"github.com/haveachin/infrared/mc"
	pk "github.com/haveachin/infrared/mc/packet"
	"github.com/haveachin/infrared/mc/protocol"
)

type Server struct {
	disconnectMessage string
	serverInfoPacket pk.Packet
}

func New(cfg ServerConfig) (*Server, error) {
	server := &Server{}
	return server, server.UpdateConfig(cfg)
}

func (server Server) HandleConn(conn mc.Conn) error {
	packet, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	handshake, err := protocol.ParseSLPHandshake(packet)
	if err != nil {
		return err
	}

	if handshake.IsStatusRequest() {
		return server.respondToSLPStatus(conn)
	} else if handshake.IsLoginRequest() {
		return server.respondToSLPLogin(conn)
	}

	return nil
}

func (server Server) respondToSLPStatus(conn mc.Conn) error {
	packet, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	if packet.ID != protocol.SLPRequestPacketID {
		return fmt.Errorf("expexted request protocol \"%d\"; got this %d", protocol.SLPRequestPacketID, packet.ID)
	}

	if err := conn.WritePacket(server.serverInfoPacket); err != nil {
		return err
	}

	packet, err = conn.ReadPacket()
	if err != nil {
		return err
	}

	if packet.ID != protocol.SLPPingPacketID {
		return fmt.Errorf("expexted ping protocol id \"%d\"; got this %d", protocol.SLPPingPacketID, packet.ID)
	}

	return conn.WritePacket(packet)
}

func (server Server) respondToSLPLogin(conn mc.Conn) error {
	packet, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	loginStart, err := protocol.ParseClientLoginStart(packet)
	if err != nil {
		return err
	}

	message := strings.Replace(server.disconnectMessage, "$username", string(loginStart.Name), -1)
	message = fmt.Sprintf("{\"text\":\"%s\"}", message)

	disconnect := protocol.LoginDisconnect{
		Reason: pk.Chat(message),
	}

	return conn.WritePacket(disconnect.Marshal())
}

func (server *Server) UpdateConfig(cfg ServerConfig) error {
	pingResponse, err := cfg.marshalPingResponse()
	if err != nil {
		return err
	}

	server.serverInfoPacket = protocol.SLPResponse{
		JSONResponse: pk.String(pingResponse),
	}.Marshal()

	server.disconnectMessage = cfg.DisconnectMessage

	return nil
}

