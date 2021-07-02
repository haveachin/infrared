package gateway

import (
	"net"

	"github.com/haveachin/infrared/connection"
)

type ListenerFactory func(addr string) (net.Listener, error)

func NewBasicListener(listener net.Listener, ch chan<- connection.HandshakeConn) BasicListener {
	return BasicListener{
		listener: listener,
		connCh:   ch,
	}
}

func NewListenerWithLogger(listener net.Listener, ch chan<- connection.HandshakeConn, logger func(err error)) BasicListener {
	return BasicListener{
		listener: listener,
		connCh:   ch,
		logger:   logger,
	}
}

type BasicListener struct {
	listener net.Listener
	connCh   chan<- connection.HandshakeConn
	logger   func(err error)
}

// The only way to stop this is when the listeners closes
func (l *BasicListener) Listen() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			l.logger(err)
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			break
		}
		remoteAddr := conn.RemoteAddr()
		c := connection.NewHandshakeConn(conn, remoteAddr)
		l.connCh <- c
	}
}
