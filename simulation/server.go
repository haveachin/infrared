package simulation

import (
	"fmt"
	"strings"

	"github.com/Tnze/go-mc/net"
	"github.com/Tnze/go-mc/net/packet"
	"github.com/haveachin/infrared/wrapper"
)

type Server struct {
	DisconnectMessage string
	PlaceholderPacket packet.Packet
}

func (server Server) RespondToSLP(conn net.Conn, handshake wrapper.SLPHandshake) error {
	if handshake.IsStatusRequest() {
		return server.RespondToSLPStatus(conn)
	} else if handshake.IsLoginRequest() {
		return server.RespondToSLPLogin(conn)
	}

	return nil
}

func (server Server) RespondToSLPStatus(conn net.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != wrapper.SLPRequestPacketID {
		return fmt.Errorf("expexted request packet \"%d\"; got this %d", wrapper.SLPRequestPacketID, pk.ID)
	}

	if err := conn.WritePacket(server.PlaceholderPacket); err != nil {
		return err
	}

	pk, err = conn.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != wrapper.SLPPingPacketID {
		return fmt.Errorf("expexted ping packet id \"%d\"; got this %d", wrapper.SLPPingPacketID, pk.ID)
	}

	return conn.WritePacket(pk)
}

func (server Server) RespondToSLPLogin(conn net.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	loginStart, err := wrapper.ParseLoginStart(pk)
	if err != nil {
		return err
	}

	message := strings.Replace(server.DisconnectMessage, "$username", string(loginStart.Name), -1)
	message = fmt.Sprintf("{\"text\":\"%s\"}", message)

	disconnect := wrapper.LoginDisconnect{
		Reason: packet.Chat(message),
	}

	return conn.WritePacket(disconnect.Marshal())
}
