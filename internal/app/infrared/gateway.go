package infrared

import (
	"errors"
	"net"
	"sync"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

type Gateway interface {
	// GetID resturns the ID of the gateway
	GetID() string
	// GetServerIDs returns the IDs of the servers
	// that are registered in that gateway
	GetServerIDs() []string
	GetServerNotFoundMessage() string
	GetLogger() logr.Logger
	SetLogger(log logr.Logger)
	GetListeners() []net.Listener
	WrapConn(net.Conn, net.Listener) net.Conn
	Close() error
}

func ListenAndServe(gw Gateway, cpnChan chan<- net.Conn) {
	logger := gw.GetLogger()
	listeners := gw.GetListeners()
	wg := sync.WaitGroup{}
	wg.Add(len(listeners))

	for _, listener := range listeners {
		logger.Info("starting to listen on",
			"network", listener.Addr().Network(),
			"address", listener.Addr(),
		)

		l := listener
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					if errors.Is(net.ErrClosed, err) {
						break
					}
					continue
				}

				keysAndValues := []interface{}{
					"gatewayId", gw.GetID(),
					"network", l.Addr().Network(),
					"listenerAddress", l.Addr(),
					"localAddress", c.LocalAddr(),
					"remoteAddress", c.RemoteAddr(),
				}

				logger.Info("new connection", keysAndValues...)
				event.Push(NewConnectionEventTopic, keysAndValues...)

				cpnChan <- gw.WrapConn(c, l)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}
