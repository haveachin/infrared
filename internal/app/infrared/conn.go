package infrared

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Edition byte

const (
	JavaEdition Edition = iota
	BedrockEdition
)

func (e Edition) String() string {
	switch e {
	case JavaEdition:
		return "java"
	case BedrockEdition:
		return "bedrock"
	default:
		return "unknown"
	}
}

// Conn is a basic Minecraft connection containing the gateway ID that it connected through
// and the Minecraft edition.
type Conn interface {
	net.Conn
	// GatewayID is the ID of the gateway that they connected through.
	GatewayID() string
	// Edition returns the Minecraft edition of this connection.
	Edition() Edition
	Pipe(c net.Conn) error
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

// ConnTunnel is a proxy tunnel between a a client and a server.
// Similar to net.Pipe
type ConnTunnel struct {
	// Conn is a ProcessedConn that will be connected to the server
	// when the ConnTunnel is started.
	Conn ProcessedConn
	// Server is the minecraft server that the Conn will be connected to.
	Server Server
}

// Start starts to proxy the Conn to the Server. This call is blocking.
func (ct ConnTunnel) Start() error {
	rc, err := ct.Server.HandleConn(ct.Conn)
	if err != nil {
		return err
	}
	defer rc.Close()

	go ct.Conn.Pipe(rc)
	rc.Pipe(ct.Conn)
	return nil
}

// Close closes both connection (client to server and server to client).
func (ct ConnTunnel) Close() error {
	return ct.Conn.Close()
}
