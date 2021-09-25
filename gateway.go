package infrared

import (
	"net"
	"sync"

	"github.com/go-logr/logr"
)

type Gateway struct {
	Binds                []string
	ReceiveProxyProtocol bool
	ReceiveRealIP        bool
	ServerIDs            []string
	Log                  logr.Logger

	listeners []net.Listener
}

func (gw *Gateway) Start(cpnChan chan<- ProcessingConn) error {
	gw.listeners = make([]net.Listener, len(gw.Binds))
	for n, bind := range gw.Binds {
		l, err := net.Listen("tcp", bind)
		if err != nil {
			return err
		}

		gw.listeners[n] = l
	}

	gw.listenAndServe(cpnChan)
	return nil
}

func (gw Gateway) wrapConn(c net.Conn) ProcessingConn {
	return ProcessingConn{
		Conn:          newConn(c),
		remoteAddr:    c.RemoteAddr(),
		proxyProtocol: gw.ReceiveProxyProtocol,
		realIP:        gw.ReceiveRealIP,
		serverIDs:     gw.ServerIDs,
	}
}

func (gw *Gateway) listenAndServe(cpnChan chan<- ProcessingConn) {
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
