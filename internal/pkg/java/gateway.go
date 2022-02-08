package java

import (
	"bufio"
	"encoding/json"
	"net"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
)

type Listener struct {
	Bind                     string
	ReceiveProxyProtocol     bool
	ReceiveRealIP            bool
	ClientTimeout            time.Duration
	ServerNotFoundMessage    string
	ServerNotFoundStatus     DialTimeoutStatusResponse
	serverNotFoundStatusJSON string

	net.Listener
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
		l, err := net.Listen("tcp", listener.Bind)
		if err != nil {
			gw.Log.Info("unable to bind listener",
				"address", listener.Bind,
			)
			continue
		}

		gw.Listeners[n].Listener = l
		gw.listeners[n] = &gw.Listeners[n]

		rJSON, err := listener.ServerNotFoundStatus.ResponseJSON()
		if err != nil {
			continue
		}

		bb, err := json.Marshal(rJSON)
		if err != nil {
			continue
		}
		gw.Listeners[n].serverNotFoundStatusJSON = string(bb)
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

func (gw *Gateway) SetLogger(log logr.Logger) {
	gw.Log = log
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
	var result error
	for _, l := range gw.listeners {
		if err := l.Close(); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}
