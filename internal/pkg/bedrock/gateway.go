package bedrock

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/sandertv/go-raknet"
	"go.uber.org/multierr"
	"go.uber.org/zap"
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
	ID                    string
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
	Logger    *zap.Logger

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		l, err := raknet.Listen(listener.Bind)
		if err != nil {
			gw.Logger.Error("unable to bind listener",
				zap.Error(err),
				zap.String("address", listener.Bind),
			)
			continue
		}
		l.PongData(listener.PingStatus.marshal(l))

		gw.Listeners[n].Listener = *l
		gw.listeners[n] = &gw.Listeners[n]
	}
}

type InfraredGateway struct {
	mu      sync.RWMutex
	Gateway Gateway
}

func (gw *InfraredGateway) ID() string {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.Gateway.ID
}

func (gw *InfraredGateway) ServerIDs() []string {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	srvIDs := make([]string, len(gw.Gateway.ServerIDs))
	copy(srvIDs, gw.Gateway.ServerIDs)
	return srvIDs
}

func (gw *InfraredGateway) SetLogger(log *zap.Logger) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.Gateway.Logger = log
}

func (gw *InfraredGateway) Logger() *zap.Logger {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.Gateway.Logger
}

func (gw *InfraredGateway) Listeners() []net.Listener {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if gw.Gateway.listeners == nil {
		gw.Gateway.initListeners()
	}

	ll := make([]net.Listener, len(gw.Gateway.ServerIDs))
	copy(ll, gw.Gateway.listeners)
	return ll
}

func (gw *InfraredGateway) WrapConn(c net.Conn, l net.Listener) net.Conn {
	listener := l.(*Listener)
	return &Conn{
		Conn:                  c.(*raknet.Conn),
		gatewayID:             gw.Gateway.ID,
		proxyProtocol:         listener.ReceiveProxyProtocol,
		serverNotFoundMessage: listener.ServerNotFoundMessage,
	}
}

func (gw *InfraredGateway) Close() error {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	var result error
	for _, l := range gw.Gateway.listeners {
		if err := l.Close(); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}
