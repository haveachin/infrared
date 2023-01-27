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
	// ID returns the ID of the gateway
	ID() string
	SetListenersManager(*ListenersManager)

	SetLogger(*zap.Logger)
	Logger() *zap.Logger
	SetEventBus(event.Bus)
	EventBus() event.Bus

	// Listeners returns a slice of all listeners that the Gateway has
	Listeners() []net.Listener
	// WrapConn extends the net.Conn interface with a implementation
	// specific struct to append extra information to the connection
	// and prepares it for processing
	WrapConn(net.Conn, net.Listener) net.Conn
	// Close closes all the underlying listeners
	Close() error
}

// ListenAndServe starts the listening process of all listeners of a Gateway
func ListenAndServe(gw Gateway, cpnChan chan<- Conn) {
	logger := gw.Logger()
	wg := sync.WaitGroup{}

	for _, listener := range gw.Listeners() {
		if listener == nil {
			continue
		}

		wg.Add(1)
		go func(l net.Listener, logger *zap.Logger) {
			for {
				c, err := l.Accept()
				if err != nil {
					if errors.Is(net.ErrClosed, err) {
						logger.Debug("listener closed")
						break
					}
					logger.Debug("listener accept error",
						zap.Error(err),
					)
					continue
				}

				conn := gw.WrapConn(c, l).(Conn)

				logger.Info("accepting new connection", logConn(c)...)

				replyChan := gw.EventBus().Request(AcceptedConnEvent{
					Conn: conn,
				}, AcceptedConnEventTopic)

				if isEventCanceled(replyChan, logger) {
					conn.Close()
					continue
				}

				cpnChan <- conn
			}
			wg.Done()
		}(listener, logger.With(logListener(listener)...))
	}

	wg.Wait()
}
