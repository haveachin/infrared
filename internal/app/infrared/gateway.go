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
	ID() string
	// GetServerIDs returns the IDs of the servers
	// that are registered in that gateway
	ServerIDs() []string
	SetLogger(logr.Logger)
	Logger() logr.Logger
	Listeners() []net.Listener
	WrapConn(net.Conn, net.Listener) net.Conn
	Close() error
}

func ListenAndServe(gw Gateway, cpnChan chan<- net.Conn) {
	logger := gw.Logger()
	listeners := gw.Listeners()
	wg := sync.WaitGroup{}
	wg.Add(len(listeners))

	for _, listener := range listeners {
		keysAndValues := []interface{}{
			"network", listener.Addr().Network(),
			"listenerAddr", listener.Addr().String(),
			"test", "test",
		}
		logger.Info("starting to listen on", keysAndValues...)

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

				keysAndValues = append(keysAndValues,
					"localAddr", c.LocalAddr().String(),
					"remoteAddr", c.RemoteAddr().String(),
					"gatewayId", gw.ID(),
				)
				logger.Info("accepting new connection", keysAndValues...)
				event.Push(NewConnectionEventTopic, keysAndValues)

				cpnChan <- gw.WrapConn(c, l)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}
