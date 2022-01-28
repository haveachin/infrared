package infrared

import (
	"io"
	"net"

	"github.com/haveachin/infrared/pkg/webhook"
)

type ProcessedConn interface {
	net.Conn
	// GatewayID is the ID of the gateway that they connected through
	GatewayID() string
	// Username returns the username of the connecting player
	Username() string
	// ServerAddr returns the exact Server Address string
	// that the client send to the server
	ServerAddr() string
	// Disconnect sends the client a disconnect message
	// and closes the connection
	Disconnect(msg string) error
}

type ConnTunnel struct {
	Conn       ProcessedConn
	RemoteConn net.Conn
	Webhooks   []webhook.Webhook
}

func (t ConnTunnel) Start() {
	defer t.Close()

	go io.Copy(t.Conn, t.RemoteConn)
	io.Copy(t.RemoteConn, t.Conn)
}

func (t ConnTunnel) Close() {
	if t.Conn != nil {
		_ = t.Conn.Close()
	}
	if t.RemoteConn != nil {
		_ = t.RemoteConn.Close()
	}
}
