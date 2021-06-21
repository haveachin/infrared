package gateway

import (
	"net"

	"github.com/haveachin/infrared/connection"
)

type ErrorLogger func(err error)
type ListenerFactory func(addr string) (net.Listener, error)

//The last argument is being used for an optional logger only the first logger will be used
func NewBasicListener(listener net.Listener, ch connection.HandshakeChannel, errorLogger ...ErrorLogger) BasicListener {
	logger := func(err error) {}
	if len(errorLogger) != 0 {
		logger = errorLogger[0]
	}
	return BasicListener{
		listener:  listener,
		connCh:    ch,
		errLogger: logger,
	}
}

type BasicListener struct {
	listener  net.Listener
	connCh    connection.HandshakeChannel
	errLogger ErrorLogger
}

// The only way to stop this is when the listeners closes
func (l *BasicListener) Listen() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			l.errLogger(err)
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
