package infrared

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type Conn interface {
	net.Conn
	// GatewayID is the ID of the gateway that they connected through
	GatewayID() string
}

// ProcessedConn is a already processed connection that waits to be handles by a server
// All methods need to be thread-safe
type ProcessedConn interface {
	Conn
	// Username returns the username of the connecting player
	Username() string
	// ServerAddr returns the exact Server Address string
	// that the client send to the server
	ServerAddr() string
	// DisconnectServerNotFound disconnects the client when the server is not found
	DisconnectServerNotFound() error
	// IsLoginRequest retruns true if the client wants to log into the server, false if they don't
	IsLoginRequest() bool
}

// ExecuteMessageTemplate injects values into a given msg template string
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

type ServerConnector interface {
}

// ConnTunnel is a proxy tunnel between a a client and a server.
// Similar to net.Pipe
type ConnTunnel struct {
	Conn   ProcessedConn
	Server Server
}

// Start starts the proxing of the tunnel
func (ct ConnTunnel) ProcessConn() error {
	rc, err := ct.Server.HandleConn(ct.Conn)
	if err != nil {
		return err
	}
	defer rc.Close()
	defer ct.Conn.Close()

	go io.Copy(ct.Conn, rc)
	_, err = io.Copy(rc, ct.Conn)
	return err
}

// Close the proxy tunnel
func (ct ConnTunnel) Close() error {
	return ct.Conn.Close()
}
