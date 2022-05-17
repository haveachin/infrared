package infrared

import (
	"errors"
	"net"
	"sync"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

// Gateway is an interface representation of a Minecraft specifc Gateways implementation.
// All methods need to be thread-safe
type Gateway interface {
	// GetID resturns the ID of the gateway
	ID() string
	// GetServerIDs returns the IDs of the servers
	// that are registered in that gateway
	ServerIDs() []string
	SetListenersManager(*ListenersManager)
	// Sets the logr.Logger implementation of the Gateway
	SetLogger(*zap.Logger)
	// Logger returns the logr.Logger implementation of the Gateway
	Logger() *zap.Logger
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
func ListenAndServe(gw Gateway, cpnChan chan<- Conn) {
	logger := gw.Logger()
	wg := sync.WaitGroup{}

	for _, listener := range gw.Listeners() {
		listenerLogger := logger.With(logListener(listener)...)
		listenerLogger.Info("starting listener")

		wg.Add(1)
		go func(l net.Listener, logger *zap.Logger) {
			for {
				c, err := l.Accept()
				if err != nil {
					if errors.Is(net.ErrClosed, err) {
						break
					}
					continue
				}

				conn := gw.WrapConn(c, l).(Conn)

				logger.Debug("accepting new connection", logConn(c)...)
				event.Push(NewConnEventTopic, NewConnEvent{
					Conn: conn,
				})

				cpnChan <- conn
			}
			wg.Done()
		}(listener, listenerLogger)
	}

	wg.Wait()
}
