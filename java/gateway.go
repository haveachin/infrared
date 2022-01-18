package java

import (
	"bufio"
	"net"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
)

type Gateway struct {
	ID                    string
	Binds                 []string
	ReceiveProxyProtocol  bool
	ReceiveRealIP         bool
	ClientTimeout         time.Duration
	ServerIDs             []string
	Log                   logr.Logger
	ServerNotFoundMessage string

	listeners []net.Listener
}

func (gw Gateway) GetID() string {
	return gw.ID
}

func (gw Gateway) GetServerIDs() []string {
	return gw.ServerIDs
}

func (gw Gateway) GetServerNotFoundMessage() string {
	return gw.ServerNotFoundMessage
}

func (gw *Gateway) SetLogger(log logr.Logger) {
	gw.Log = log
}

func (gw Gateway) GetLogger() logr.Logger {
	return gw.Log
}

func (gw *Gateway) initListeners() {
	gw.listeners = make([]net.Listener, len(gw.Binds))
	for n, bind := range gw.Binds {
		l, err := net.Listen("tcp", bind)
		if err != nil {
			gw.Log.Info("unable to bind listener",
				"address", bind,
			)
			continue
		}

		gw.listeners[n] = l
	}
}

func (gw *Gateway) GetListeners() []net.Listener {
	if gw.listeners == nil {
		gw.initListeners()
	}

	return gw.listeners
}

func (gw Gateway) WrapConn(c net.Conn, l net.Listener) net.Conn {
	return &Conn{
		Conn:          c,
		r:             bufio.NewReader(c),
		w:             c,
		proxyProtocol: gw.ReceiveProxyProtocol,
		realIP:        gw.ReceiveRealIP,
		gatewayID:     gw.ID,
	}
}

func (gw *Gateway) Close() error {
	var result error
	for _, l := range gw.listeners {
		if err := l.Close(); err != nil {
			result = multierr.Append(result, err)
		}
	}
	return result
}
