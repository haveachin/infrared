package java

import (
	"bufio"
	"net"
	"sync"
	"time"

	"github.com/go-logr/logr"
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

func (gw *Gateway) ListenAndServe(cpnChan chan<- net.Conn) error {
	gw.listeners = make([]net.Listener, len(gw.Binds))
	for n, bind := range gw.Binds {
		gw.Log.Info("start listener",
			"bind", bind,
		)

		l, err := net.Listen("tcp", bind)
		if err != nil {
			return err
		}

		gw.listeners[n] = l
	}

	gw.listenAndServe(cpnChan)
	return nil
}

func (gw Gateway) wrapConn(c net.Conn) *Conn {
	return &Conn{
		Conn:          c,
		r:             bufio.NewReader(c),
		w:             c,
		proxyProtocol: gw.ReceiveProxyProtocol,
		realIP:        gw.ReceiveRealIP,
		gatewayID:     gw.ID,
	}
}

func (gw *Gateway) listenAndServe(cpnChan chan<- net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(len(gw.listeners))

	for _, listener := range gw.listeners {
		l := listener
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					break
				}

				gw.Log.Info("connected",
					"remoteAddress", c.RemoteAddr(),
				)

				cpnChan <- gw.wrapConn(c)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}
