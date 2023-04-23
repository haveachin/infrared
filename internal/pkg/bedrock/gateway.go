package bedrock

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/pires/go-proxyproto"
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
	Compression           packet.Compression

	net.Listener
}

type Gateway struct {
	ID               string
	ListenersManager *infrared.ListenersManager
	Listeners        []Listener
	Logger           *zap.Logger
	EventBus         event.Bus

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		logger := gw.Logger.With(
			zap.String("address", listener.Bind),
		)

		l, err := gw.ListenersManager.Listen(listener.Bind, func(l net.Listener) {
			pl := l.(*proxyProtocolListener)
			rl := pl.Listener
			pong := listener.PingStatus.marshal(rl)
			rl.PongData(pong)
			pl.proxyProtocolPacketConn.active = listener.ReceiveProxyProtocol

			if listener.ReceiveProxyProtocol {
				logger.Warn("receiving proxy protocol")
			}
		})
		if err != nil {
			logger.Warn("unable to bind listener",
				zap.Error(err),
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

func (gw *InfraredGateway) EventBus() event.Bus {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.gateway.EventBus
}

func (gw *InfraredGateway) SetEventBus(bus event.Bus) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.gateway.EventBus = bus
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
		Conn:          c.(*raknet.Conn),
		decoder:       packet.NewDecoder(c),
		encoder:       packet.NewEncoder(c),
		gatewayID:     gw.gateway.ID,
		proxyProtocol: listener.ReceiveProxyProtocol,
		serverNotFoundDisconnector: PlayerDisconnecter{
			message: listener.ServerNotFoundMessage,
		},
		compression: listener.Compression,
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

type proxyProtocolListener struct {
	*raknet.Listener
	proxyProtocolPacketConn *proxyProtocolPacketConn
}

type proxyProtocolPacketConn struct {
	net.PacketConn

	mu      sync.Mutex
	conns   map[string]net.Addr
	timeout map[string]time.Time
	active  bool
}

func (pc *proxyProtocolPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	if !pc.active {
		return pc.PacketConn.ReadFrom(p)
	}

	payload := make([]byte, len(p))
	n, addr, err := pc.PacketConn.ReadFrom(payload)
	if err != nil {
		return 0, nil, err
	}
	payload = payload[:n]

	addrString := addr.String()
	payloadAt := time.Now()

	lastPayloadAt, found := pc.timeout[addrString]
	if found && lastPayloadAt.Add(30*time.Second).Before(payloadAt) {
		delete(pc.conns, addrString)
	}
	pc.timeout[addrString] = payloadAt

	realAddr, found := pc.conns[addrString]
	if !found {
		buf := bytes.NewBuffer(payload)
		header, err := proxyproto.Read(bufio.NewReader(buf))
		if err != nil {
			return 0, nil, err
		}
		realAddr = header.SourceAddr
		pc.conns[addrString] = realAddr
		payload = buf.Bytes()
	}

	copy(p, payload)
	return len(payload), realAddr, nil
}

func (pc *proxyProtocolPacketConn) ListenPacket(network, address string) (net.PacketConn, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	conn, err := net.ListenPacket(network, address)
	if err != nil {
		return nil, err
	}
	pc.PacketConn = conn
	pc.conns = map[string]net.Addr{}
	pc.timeout = map[string]time.Time{}
	return pc, nil
}
