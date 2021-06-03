package connection

import (
	"errors"
	"io"
	"net"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrNoNameYet = errors.New("we dont have the name of this player yet")
)

type ServerConnFactory func(addr string) (ServerConnection, error)
type HSConnFactory func(conn Connection, remoteAddr net.Addr) (HSConnection, error)

type RequestType int8

const (
	UnknownRequest RequestType = 0
	StatusRequest  RequestType = 1
	LoginRequest   RequestType = 2
)

// probably needs a better name since its not only used for piping the connection
type PipeConnection interface {
	getConn() ByteConnection
}

type ByteConnection interface {
	io.Writer
	io.Reader
	io.Closer
}

type Connection interface {
	infrared.PacketWriter
	infrared.PacketReader
}

type HSConnection interface {
	Connection
	Handshake() handshaking.ServerBoundHandshake
	HsPk() protocol.Packet

	SetHsPk(pk protocol.Packet)
	SetHandshake(hs handshaking.ServerBoundHandshake)

	RemoteAddr() net.Addr
}

type LoginConnection interface {
	HSConnection
	PipeConnection
}

type StatusConnection interface {
	HSConnection
}

type ServerConnection interface {
	PipeConnection
	Connection
}
