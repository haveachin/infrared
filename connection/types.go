package connection

import (
	"errors"
	"net"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrCantGetHSPacket = errors.New("cant get handshake packet from caller")
	ErrNoNameYet       = errors.New("we dont have the name of this player yet")
)

type ServerConnFactory func(addr string) (ServerConnection, error)
type HSConnFactory func(conn Connection, remoteAddr net.Addr) (HSConnection, error)

type RequestType int8

const (
	UnknownRequest RequestType = 0
	StatusRequest  RequestType = 1
	LoginRequest   RequestType = 2
)

type PipeConnection interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
}

type Connection interface {
	infrared.PacketWriter
	infrared.PacketReader
	PipeConnection
}

type HSConnection interface {
	Connection
	RemoteAddressConnection
	Handshake() (handshaking.ServerBoundHandshake, error)
	HsPk() (protocol.Packet, error)
}

type GatewayConnection interface {
	ServerAddr() string
	RemoteAddressConnection
}

type LoginConnection interface {
	HSConnection
	Name() (string, error)
	LoginStart() (protocol.Packet, error) // Need more work
}

type StatusConnection interface {
	HSConnection
}

type ServerConnection interface {
	PipeConnection
	Status(pk protocol.Packet) (protocol.Packet, error)
	SendPK(pk protocol.Packet) error
}

type RemoteAddressConnection interface {
	RemoteAddr() net.Addr
}
