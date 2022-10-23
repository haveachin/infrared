package bedrock

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
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

	net.Listener
}

type Gateway struct {
	ID               string
	Compression      packet.Compression
	ListenersManager *infrared.ListenersManager
	Listeners        []Listener
	Logger           *zap.Logger

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		l, err := gw.ListenersManager.Listen(listener.Bind, func(l net.Listener) {
			rl := l.(*raknet.Listener)
			pong := listener.PingStatus.marshal(rl)
			rl.PongData(pong)
		})
		if err != nil {
			gw.Logger.Error("unable to bind listener",
				zap.Error(err),
				zap.String("address", listener.Bind),
			)
			continue
		}

		gw.Listeners[n].Listener = l
		gw.listeners[n] = &gw.Listeners[n]
	}
}

type InfraredGateway struct {
	mu      sync.RWMutex
	gateway Gateway
}

func (gw *InfraredGateway) ID() string {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.gateway.ID
}

func (gw *InfraredGateway) SetListenersManager(lm *infrared.ListenersManager) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.gateway.ListenersManager = lm

	if gw.gateway.listeners == nil {
		gw.gateway.initListeners()
	}
}

func (gw *InfraredGateway) SetLogger(log *zap.Logger) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.gateway.Logger = log
}

func (gw *InfraredGateway) Logger() *zap.Logger {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.gateway.Logger
}

func (gw *InfraredGateway) Listeners() []net.Listener {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	ll := make([]net.Listener, len(gw.gateway.listeners))
	copy(ll, gw.gateway.listeners)
	return ll
}

func (gw *InfraredGateway) WrapConn(c net.Conn, l net.Listener) net.Conn {
	listener := l.(*Listener)
	return &Conn{
		Conn:                  c.(*raknet.Conn),
		decoder:               packet.NewDecoder(c),
		encoder:               packet.NewEncoder(c),
		gatewayID:             gw.gateway.ID,
		proxyProtocol:         listener.ReceiveProxyProtocol,
		serverNotFoundMessage: listener.ServerNotFoundMessage,
		compression:           gw.gateway.Compression,
	}
}

func (gw *InfraredGateway) Close() error {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	var result error
	for _, l := range gw.gateway.listeners {
		if err := l.Close(); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}
