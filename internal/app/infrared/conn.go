package infrared

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"
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
	// DisconnectServerNotFound disconnects the client when the server is not found
	DisconnectServerNotFound() error
	IsLoginRequest() bool
}

func ExecuteMessageTemplate(msg string, pc ProcessedConn) string {
	tmpls := map[string]string{
		"username":      pc.Username(),
		"currentTime":   time.Now().Format(time.RFC822),
		"remoteAddress": pc.RemoteAddr().String(),
		"localAddress":  pc.LocalAddr().String(),
		"serverDomain":  pc.ServerAddr(),
		"gatewayId":     pc.GatewayID(),
	}

	for k, v := range tmpls {
		msg = strings.Replace(msg, fmt.Sprintf("{{%s}}", k), v, -1)
	}

	return msg
}

type ConnTunnel struct {
	Conn       ProcessedConn
	RemoteConn net.Conn
	WebhookIds []string
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
