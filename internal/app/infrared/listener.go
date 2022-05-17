package infrared

import (
	"fmt"
	"log"
	"net"
	"sync"

	"go.uber.org/atomic"
)

type listener struct {
	listener net.Listener
	connCh   <-chan net.Conn
	errCh    chan error
	onClose  func() error
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-l.connCh:
		if !ok {
			return nil, net.ErrClosed
		}
		return c, nil
	case err := <-l.errCh:
		return nil, err
	}
}

func (l *listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *listener) Close() error {
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
	return &listener{
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

	listeners map[string]*managedListener
	mu        sync.Mutex
}

func (m *ListenersManager) Listen(addr string) (net.Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	addr = fmt.Sprintf("%s:%s", host, port)
	ml, ok := m.listeners[addr]
	if ok {
		return ml.newSubscriber(), nil
	}

	l, err := m.New(addr)
	if err != nil {
		return nil, err
	}

	ml = newManagedListener(l)
	m.listeners[addr] = ml
	return ml.newSubscriber(), nil
}

func (m *ListenersManager) clean() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, l := range m.listeners {
		if l.subscriber.Load() > 0 {
			continue
		}

		log.Println("Closing " + l.Addr().String())
		l.Close()
	}
}
