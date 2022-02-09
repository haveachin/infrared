package bedrock

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/sandertv/go-raknet"
)

type PingStatus struct {
	Edition         string
	ProtocolVersion int
	VersionName     string
	PlayerCount     int
	MaxPlayerCount  int
	GameMode        string
	GameModeNumeric int
	MOTD            string
}

func (p PingStatus) marshal(l *raknet.Listener) []byte {
	motd := strings.Split(p.MOTD, "\n")
	motd1 := motd[0]
	motd2 := ""
	if len(motd) > 1 {
		motd2 = motd[1]
	}

	port := l.Addr().(*net.UDPAddr).Port
	return []byte(fmt.Sprintf("%v;%v;%v;%v;%v;%v;%v;%v;%v;%v;%v;%v;",
		p.Edition, motd1, p.ProtocolVersion, p.VersionName, p.PlayerCount, p.MaxPlayerCount,
		l.ID(), motd2, p.GameMode, p.GameModeNumeric, port, port))
}

type Listener struct {
	Bind                  string
	ReceiveProxyProtocol  bool
	PingStatus            PingStatus
	ServerNotFoundMessage string

	raknet.Listener
}

type Gateway struct {
	ID        string
	Listeners []Listener
	ServerIDs []string
	Log       logr.Logger

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		l, err := raknet.Listen(listener.Bind)
		if err != nil {
			gw.Log.Info("unable to bind listener",
				"address", listener.Bind,
			)
			continue
		}
		l.PongData(listener.PingStatus.marshal(l))

		gw.Listeners[n].Listener = *l
		gw.listeners[n] = &gw.Listeners[n]
	}
}

type InfraredGateway struct {
	Gateway
}

func (gw InfraredGateway) ID() string {
	return gw.Gateway.ID
}

func (gw InfraredGateway) ServerIDs() []string {
	return gw.Gateway.ServerIDs
}

func (gw *InfraredGateway) SetLogger(log logr.Logger) {
	gw.Gateway.Log = log
}

func (gw InfraredGateway) Logger() logr.Logger {
	return gw.Gateway.Log
}

func (gw *InfraredGateway) Listeners() []net.Listener {
	if gw.listeners == nil {
		gw.initListeners()
	}

	return gw.listeners
}

func (gw InfraredGateway) WrapConn(c net.Conn, l net.Listener) net.Conn {
	listener := l.(*Listener)
	return &Conn{
		Conn:                  c.(*raknet.Conn),
		gatewayID:             gw.Gateway.ID,
		proxyProtocol:         listener.ReceiveProxyProtocol,
		serverNotFoundMessage: listener.ServerNotFoundMessage,
	}
}

func (gw *InfraredGateway) Close() error {
	for _, l := range gw.listeners {
		if err := l.Close(); err != nil {
			return err
		}
	}
	return nil
}
