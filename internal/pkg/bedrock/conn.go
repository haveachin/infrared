package bedrock

import (
	"net"

	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
	"github.com/sandertv/go-raknet"
)

// Conn is a minecraft Connection
type Conn struct {
	*raknet.Conn

	gatewayID     string
	proxyProtocol bool
	realIP        bool
}

type ProcessedConn struct {
	*Conn
	readBytes     []byte
	remoteAddr    net.Addr
	serverAddr    string
	username      string
	proxyProtocol bool
}

func (c ProcessedConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c ProcessedConn) GatewayID() string {
	return c.gatewayID
}

func (c ProcessedConn) Username() string {
	return c.username
}

func (c ProcessedConn) ServerAddr() string {
	return c.serverAddr
}

func (c ProcessedConn) Disconnect(msg string) error {
	defer c.Close()
	pk := protocol.Disconnect{
		HideDisconnectionScreen: msg == "",
		Message:                 msg,
	}

	b, err := protocol.MarshalPacket(&pk)
	if err != nil {
		return err
	}

	if _, err := c.Write(b); err != nil {
		return err
	}

	return nil
}
