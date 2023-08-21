package infrared

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

var lMgr = listenersManager{
	New: func(addr string) (net.Listener, error) {
		return net.Listen("tcp", addr)
	},
	listeners: make(map[string]*managedListener),
}

type ListenerConfigFunc func(cfg *ListenerConfig)

func WithListenerConfig(c ListenerConfig) ListenerConfigFunc {
	return func(cfg *ListenerConfig) {
		*cfg = c
	}
}

func WithListenerBind(bind string) ListenerConfigFunc {
	return func(cfg *ListenerConfig) {
		cfg.Bind = bind
	}
}

type ListenerConfig struct {
	Bind string `yaml:"bind"`
}

type Listener struct {
	net.Listener
	cfg ListenerConfig
}

func NewListener(fns ...ListenerConfigFunc) (*Listener, error) {
	var cfg ListenerConfig
	for _, fn := range fns {
		fn(&cfg)
	}

	l, err := lMgr.Listen(cfg.Bind)
	if err != nil {
		return nil, err
	}

	return &Listener{
		Listener: l,
		cfg:      cfg,
	}, nil
}

type listener struct {
	listener net.Listener
	connCh   <-chan net.Conn
	errCh    chan error
	onAccept []func(c net.Conn) (net.Conn, error)
	onClose  func() error
}

func (l *listener) Accept() (net.Conn, error) {
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

func (l *listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *listener) Close() error {
	l.errCh <- net.ErrClosed
	return l.onClose()
}

type managedListener struct {
	net.Listener
	connChan   chan net.Conn
	errChan    chan error
	subscriber atomic.Int32
}

func newManagedListener(l net.Listener) *managedListener {
	connChan := make(chan net.Conn)
	errChan := make(chan error)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				errChan <- err
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}

			connChan <- c
		}
	}()

	return &managedListener{
		Listener: l,
		connChan: connChan,
		errChan:  errChan,
	}
}

func (ml *managedListener) newSubscriber() net.Listener {
	ml.subscriber.Add(1)
	return &listener{
		listener: ml.Listener,
		connCh:   ml.connChan,
		errCh:    ml.errChan,
		onClose: func() error {
			ml.subscriber.Add(-1)
			return nil
		},
	}
}

type listenerBuilder func(addr string) (net.Listener, error)

type listenersManager struct {
	New listenerBuilder

	listeners map[string]*managedListener
	mu        sync.Mutex
}

func (lm *listenersManager) Listen(addr string) (net.Listener, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	addr, err := formatAddr(addr)
	if err != nil {
		return nil, err
	}

	ml, ok := lm.listeners[addr]
	if ok {
		return ml.newSubscriber(), nil
	}

	l, err := lm.New(addr)
	if err != nil {
		return nil, err
	}

	ml = newManagedListener(l)
	lm.listeners[addr] = ml
	return ml.newSubscriber(), nil
}

func (lm *listenersManager) prune() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for addr, l := range lm.listeners {
		if l.subscriber.Load() > 0 {
			continue
		}
		l.Close()
		delete(lm.listeners, addr)
	}
}

func formatAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", host, port), nil
}
