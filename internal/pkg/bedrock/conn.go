package bedrock

import (
	"net"

	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
	"github.com/sandertv/go-raknet"
)

// Conn is a minecraft Connection
type Conn struct {
	*raknet.Conn

	gatewayID             string
	proxyProtocol         bool
	realIP                bool
	serverNotFoundMessage string
}

type ProcessedConn struct {
	*Conn
	readBytes     []byte
	remoteAddr    net.Addr
	serverAddr    string
	username      string
	proxyProtocol bool
}

func (pc ProcessedConn) RemoteAddr() net.Addr {
	return pc.remoteAddr
}

func (pc ProcessedConn) GatewayID() string {
	return pc.gatewayID
}

func (pc ProcessedConn) Username() string {
	return pc.username
}

func (pc ProcessedConn) ServerAddr() string {
	return pc.serverAddr
}

func (pc ProcessedConn) DisconnectServerNotFound() error {
	return pc.disconnect(pc.serverNotFoundMessage)
}

func (pc ProcessedConn) disconnect(msg string) error {
	defer pc.Close()
	pk := protocol.Disconnect{
		HideDisconnectionScreen: msg == "",
		Message:                 msg,
	}

	b, err := protocol.MarshalPacket(&pk)
	if err != nil {
		return err
	}

	if _, err := pc.Write(b); err != nil {
		return err
	}

	return nil
}
