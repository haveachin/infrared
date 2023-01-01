package infrared

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/haveachin/infrared/pkg/event"
)

type Edition string

const (
	JavaEdition    Edition = "java"
	BedrockEdition Edition = "bedrock"
)

func Editions() []Edition {
	return []Edition{
		JavaEdition,
		BedrockEdition,
	}
}

func EditionFromString(s string) (Edition, error) {
	switch strings.ToLower(s) {
	case string(JavaEdition):
		return JavaEdition, nil
	case string(BedrockEdition):
		return BedrockEdition, nil
	}
	return Edition(""), fmt.Errorf("%q is not a valid edition", s)
}

func (e Edition) String() string {
	return string(e)
}

// Conn is a basic Minecraft connection containing the gateway ID that it connected through
// and the Minecraft edition.
type Conn interface {
	net.Conn
	// GatewayID is the ID of the gateway that they connected through.
	GatewayID() string
	// Edition returns the Minecraft edition of this connection.
	Edition() Edition
	Pipe(c net.Conn) (n int64, err error)
}

// Player is a already processed connection that waits to be handles by a server
// All methods need to be thread-safe
type Player interface {
	Conn
	// Username returns the username of the connecting player
	Username() string
	// ServerAddr returns the exact Server Address string
	// that the client send to the server
	ServerAddr() string
	// DisconnectServerNotFound disconnects the client when the server is not found
	DisconnectServerNotFound() error
	// IsLoginRequest returns true if the client wants to log into the server, false if they don't
	IsLoginRequest() bool
	// RemoteIP returns the remote IP address of the player
	RemoteIP() net.IP
}

type PlayerDisconnecter interface {
	DisconnectPlayer(Player, ...MessageOption) error
}

type multiPlayerDisconnecter struct {
	disconnecters map[Edition]PlayerDisconnecter
}

func NewMultiPlayerDisconnecter(disconnecter map[Edition]PlayerDisconnecter) PlayerDisconnecter {
	return multiPlayerDisconnecter{
		disconnecters: disconnecter,
	}
}

func (d multiPlayerDisconnecter) DisconnectPlayer(p Player, opts ...MessageOption) error {
	disconnecter, ok := d.disconnecters[p.Edition()]
	if !ok {
		return errors.New("disconncter for edition %q not registered")
	}

	return disconnecter.DisconnectPlayer(p, opts...)
}

type MessageOption func(string) string

func ApplyTemplates(tmpls ...map[string]string) MessageOption {
	return func(msg string) string {
		for _, tmpl := range tmpls {
			msg = ExecuteMessageTemplate(msg, tmpl)
		}
		return msg
	}
}

func ExecuteMessageTemplate(msg string, tmpls map[string]string) string {
	for k, v := range tmpls {
		msg = strings.Replace(msg, fmt.Sprintf("{{%s}}", k), v, -1)
	}

	return msg
}

func TimeMessageTemplates() map[string]string {
	return map[string]string{
		"currentTime": time.Now().Format(time.RFC822),
	}
}

func PlayerMessageTemplates(p Player) map[string]string {
	return map[string]string{
		"username":      p.Username(),
		"remoteAddress": p.RemoteAddr().String(),
		"localAddress":  p.LocalAddr().String(),
		"serverDomain":  p.ServerAddr(),
		"gatewayId":     p.GatewayID(),
	}
}

func ServerMessageTemplate(s Server) map[string]string {
	return map[string]string{
		"serverId": s.ID(),
	}
}

// ConnTunnel is a proxy tunnel between a a client and a server.
// Similar to net.Pipe
type ConnTunnel struct {
	// Conn that will be connected to the server
	// when the ConnTunnel is started.
	Conn Player
	// Server is the minecraft server that the Conn will be connected to.
	Server Server
	// MatchedDomain is the domain that the client matched when resolving
	// the server that it requested.
	MatchedDomain string
	EventBus      event.Bus
}

// Start starts to proxy the Conn to the Server. This call is blocking.
func (ct ConnTunnel) Start() (int64, error) {
	rc, err := ct.Server.HandleConn(ct.Conn)
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	var consumedBytes int64
	go func() {
		n, _ := ct.Conn.Pipe(rc)
		consumedBytes += n
	}()
	n, _ := rc.Pipe(ct.Conn)
	consumedBytes += n
	return consumedBytes, nil
}

// Close closes both connection (client to server and server to client).
func (ct ConnTunnel) Close() error {
	return ct.Conn.Close()
}
