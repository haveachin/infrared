package infrared

import (
	"fmt"
	"net"
	"sync"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Listener struct {
	listener net.Listener
	connCh   <-chan net.Conn
	errCh    chan error
	onAccept []func(c net.Conn) (net.Conn, error)
	onClose  func() error
}

func (l *Listener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-l.connCh:
		if !ok {
			return nil, net.ErrClosed
		}

		var err error
		for _, onAccept := range l.onAccept {
			c, err = onAccept(c)
			if err != nil {
				return nil, err
			}
		}

		return c, nil
	case err := <-l.errCh:
		return nil, err
	}
}

func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *Listener) Close() error {
	l.errCh <- net.ErrClosed
	return l.onClose()
}

type managedListener struct {
	net.Listener
	connCh     chan net.Conn
	subscriber atomic.Uint32
}

func newManagedListener(l net.Listener) *managedListener {
	connCh := make(chan net.Conn)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}

			connCh <- c
		}
	}()

	return &managedListener{
		Listener: l,
		connCh:   connCh,
	}
}

func (ml *managedListener) newSubscriber() net.Listener {
	ml.subscriber.Inc()
	return &Listener{
		listener: ml.Listener,
		connCh:   ml.connCh,
		errCh:    make(chan error),
		onClose: func() error {
			ml.subscriber.Dec()
			return nil
		},
	}
}

type ListenerBuilder func(addr string) (net.Listener, error)

type ListenersManager struct {
	New ListenerBuilder

	logger    *zap.Logger
	listeners map[string]*managedListener
	mu        sync.Mutex
}

type ListenerOption func(l net.Listener)

func (m *ListenersManager) Listen(addr string, opts ...ListenerOption) (net.Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	addr, err := formatAddr(addr)
	if err != nil {
		return nil, err
	}

	ml, ok := m.listeners[addr]
	if ok {
		for _, opt := range opts {
			opt(ml.Listener)
		}
		return ml.newSubscriber(), nil
	}

	l, err := m.New(addr)
	if err != nil {
		return nil, err
	}
	m.logger.Info("starting listener",
		zap.String("address", addr),
		zap.String("network", l.Addr().Network()),
	)

	for _, opt := range opts {
		opt(l)
	}

	ml = newManagedListener(l)
	m.listeners[addr] = ml
	return ml.newSubscriber(), nil
}

func (m *ListenersManager) clean() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for addr, l := range m.listeners {
		if l.subscriber.Load() > 0 {
			continue
		}

		m.logger.Info("closing listener",
			zap.String("address", l.Addr().String()),
		)
		l.Close()
		delete(m.listeners, addr)
	}
}

func formatAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", host, port), nil
}
