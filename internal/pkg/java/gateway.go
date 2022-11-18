package java

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"

	"github.com/haveachin/infrared/internal/app/infrared"
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
	ServerNotFoundStatus     DialTimeoutStatusResponse
	serverNotFoundStatusJSON string

	net.Listener
}

type Gateway struct {
	ID               string
	ListenersManager *infrared.ListenersManager
	Listeners        []Listener
	Logger           *zap.Logger

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		l, err := gw.ListenersManager.Listen(listener.Bind, func(l net.Listener) {
			pl := l.(*ProxyProtocolListener)
			pl.active = listener.ReceiveProxyProtocol
		})
		if err != nil {
			gw.Logger.Warn("unable to bind listener",
				zap.Error(err),
				zap.String("address", listener.Bind),
			)
		}

		gw.Listeners[n].Listener = l
		gw.listeners[n] = &gw.Listeners[n]

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
		Conn:                     c,
		r:                        bufio.NewReader(c),
		w:                        c,
		realIP:                   listener.ReceiveRealIP,
		gatewayID:                gw.gateway.ID,
		serverNotFoundMessage:    listener.ServerNotFoundMessage,
		serverNotFoundStatusJSON: listener.serverNotFoundStatusJSON,
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
	active bool
}

func (l ProxyProtocolListener) Accept() (net.Conn, error) {
	if !l.active {
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

	return &ProxyProtocolConn{
		Conn:     c,
		realAddr: header.SourceAddr,
	}, nil
}
