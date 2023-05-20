package java

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/pires/go-proxyproto"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type Listener struct {
	ID                       string
	Bind                     string
	ReceiveProxyProtocol     bool
	ReceiveRealIP            bool
	ServerNotFoundMessage    string
	ServerNotFoundStatus     ServerStatusResponse
	serverNotFoundStatusJSON string

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
	gw.listeners = make([]net.Listener, 0, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		logger := gw.Logger.With(
			zap.String("address", listener.Bind),
		)

		l, err := gw.ListenersManager.Listen(listener.Bind, func(l net.Listener) {
			pl := l.(*ProxyProtocolListener)
			pl.active.Store(listener.ReceiveProxyProtocol)

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
		gw.listeners = append(gw.listeners, &gw.Listeners[n])

		rJSON := listener.ServerNotFoundStatus.ResponseJSON()
		bb, err := json.Marshal(rJSON)
		if err != nil {
			continue
		}
		gw.Listeners[n].serverNotFoundStatusJSON = string(bb)
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
		Conn:      c,
		r:         bufio.NewReader(c),
		w:         c,
		realIP:    listener.ReceiveRealIP,
		gatewayID: gw.gateway.ID,
		serverNotFoundDisconnector: PlayerDisconnecter{
			message:    listener.ServerNotFoundMessage,
			statusJSON: listener.serverNotFoundStatusJSON,
		},
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

type ProxyProtocolConn struct {
	net.Conn
	realAddr net.Addr
}

func (c ProxyProtocolConn) RemoteAddr() net.Addr {
	return c.realAddr
}

type ProxyProtocolListener struct {
	net.Listener

	active atomic.Bool
}

func (l *ProxyProtocolListener) Accept() (net.Conn, error) {
	if !l.active.Load() {
		return l.Listener.Accept()
	}

	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	header, err := proxyproto.Read(bufio.NewReader(c))
	if err != nil {
		return nil, err
	}

	if header.SourceAddr == nil {
		c.Close()
		return nil, errors.New("no source addr in proxy header")
	}

	return ProxyProtocolConn{
		Conn:     c,
		realAddr: header.SourceAddr,
	}, nil
}
