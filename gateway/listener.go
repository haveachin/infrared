package gateway

import (
	"errors"
	"net"

	"github.com/haveachin/infrared/connection"
)

var (
	ErrCantStartOutListener = errors.New("failed to start outer listener")
	ErrCantStartListener    = errors.New("failed to start listener")
)

type OuterListener interface {
	Start() error
	Accept() connection.Connection
}

func CreateBasicOuterListener(addr string) OuterListener {
	return &BasicOuterListener{addr: addr}
}

type BasicOuterListener struct {
	net.Listener
	addr string
}

func (l *BasicOuterListener) Start() error {
	netL, err := net.Listen("tcp", l.addr) // TODO: look into more specific error test way
	if err != nil {
		return ErrCantStartOutListener
	}
	l.Listener = netL
	return nil

}

func (l *BasicOuterListener) Accept() connection.Connection {
	conn, _ := l.Listener.Accept() // Err needs test before it can be added
	return connection.CreateBasicConnection(conn)
}

type BasicListener struct {
	OutListener OuterListener
	Gw          Gateway
}

func (l *BasicListener) Listen() error {
	err := l.OutListener.Start()
	if err != nil {
		return ErrCantStartListener
	}
	conn := l.OutListener.Accept()
	pConn := conn.(connection.PlayerConnection)
	l.Gw.HandleConnection(pConn)
	return nil
}
