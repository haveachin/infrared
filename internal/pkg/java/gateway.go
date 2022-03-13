package java

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"

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
	ID        string
	Listeners []Listener
	ServerIDs []string
	Logger    *zap.Logger

	listeners []net.Listener
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Listeners))
	for n, listener := range gw.Listeners {
		l, err := net.Listen("tcp", listener.Bind)
		if err != nil {
			gw.Logger.Fatal("unable to bind listener",
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
		Conn:                     c,
		r:                        bufio.NewReader(c),
		w:                        c,
		proxyProtocol:            listener.ReceiveProxyProtocol,
		realIP:                   listener.ReceiveRealIP,
		gatewayID:                gw.Gateway.ID,
		serverNotFoundMessage:    listener.ServerNotFoundMessage,
		serverNotFoundStatusJSON: listener.serverNotFoundStatusJSON,
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
