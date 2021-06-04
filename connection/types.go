package connection

import (
	"errors"
	"io"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrNoNameYet = errors.New("we dont have the name of this player yet")
)

type ServerConnFactory func(addr string) (ServerConn, error)
type HandshakeConnFactory func(conn Conn, remoteAddr net.Addr) (HandshakeConn, error)

type RequestType int8

const (
	UnknownRequest RequestType = 0
	StatusRequest  RequestType = 1
	LoginRequest   RequestType = 2
)

// probably needs a better name since its not only used for piping the connection
type PipeConn interface {
	conn() ByteConnection
}

type ByteConnection interface {
	io.Writer
	io.Reader
	io.Closer
}

type Conn interface {
	WritePacket(p protocol.Packet) error
	ReadPacket() (protocol.Packet, error)
}

type HandshakeConn interface {
	Conn
	Handshake() handshaking.ServerBoundHandshake
	HandshakePacket() protocol.Packet

	SetHandshakePacket(pk protocol.Packet)
	SetHandshake(hs handshaking.ServerBoundHandshake)

	RemoteAddr() net.Addr
}

type LoginConn interface {
	HandshakeConn
	PipeConn
}

type StatusConn interface {
	HandshakeConn
}

type ServerConn interface {
	PipeConn
	Conn
}
