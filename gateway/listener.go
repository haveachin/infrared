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
	Accept() (connection.Connection, net.Addr)
}

func CreateBasicOuterListener(addr string) OuterListener {
	return &BasicOuterListener{addr: addr}
}

type BasicOuterListener struct {
	net.Listener
	addr string
}

func (l *BasicOuterListener) Start() error {
	netL, err := net.Listen("tcp", l.addr)
	if err != nil {
		return ErrCantStartOutListener // TODO: look into ways to test this
	}
	l.Listener = netL
	return nil

}

func (l *BasicOuterListener) Accept() (connection.Connection, net.Addr) {
	conn, _ := l.Listener.Accept() // Err needs test before it can be added
	return connection.CreateBasicConnection(conn), conn.RemoteAddr()
}

type BasicListener struct {
	OutListener OuterListener
	ConnCh      chan<- connection.GatewayConnection
}

func (l *BasicListener) Listen() error {
	err := l.OutListener.Start()
	if err != nil {
		return ErrCantStartListener
	}
	for {
		conn, remoteAddr := l.OutListener.Accept()
		c := connection.CreateBasicPlayerConnection(conn, remoteAddr)
		l.ConnCh <- c
	}
}
