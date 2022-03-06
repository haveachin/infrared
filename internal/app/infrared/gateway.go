package infrared

import (
	"errors"
	"net"
	"sync"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

// Gateway is an interface representation of a Minecraft specifc Gateways implementation.
// All methods need to be thread-safe
type Gateway interface {
	// GetID resturns the ID of the gateway
	ID() string
	// GetServerIDs returns the IDs of the servers
	// that are registered in that gateway
	ServerIDs() []string
	// Sets the logr.Logger implementation of the Gateway
	SetLogger(logr.Logger)
	// Logger returns the logr.Logger implementation of the Gateway
	Logger() logr.Logger
	// Returns alls the listener that the Gateway has
	Listeners() []net.Listener
	// WrapConn extends the net.Conn interface with a implementation
	// specific struct to append extra information to the connection
	// and prepares it for processing
	WrapConn(net.Conn, net.Listener) net.Conn
	// Close closes all the underlying listeners
	Close() error
}

// ListenAndServe starts the listening process of all listernes of a Gateway
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
