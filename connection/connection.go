package connection

import (
	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/protocol"
)

type RequestType int8

const (
	UnknownRequest int8 = 0
	StatusRequest  int8 = 1
	LoginRequest   int8 = 2
)

type Connection interface {
	infrared.PacketWriter
	infrared.PacketReader
}

type PlayerConnection interface {
	// Pick one of two ways i think
	isRequestType(requestID int8) bool
	requestType() RequestType
}

type LoginConnection interface {
	HS() (protocol.Packet, error)
	LoginStart() (protocol.Packet, error)
}

type StatusConnection interface {
	SendStatus(status protocol.Packet) error
}

// ServerConenction is the stuff that creates a connection with the real server
type ServerConnection interface {
	Status() (protocol.Packet, error)
	SendPK(pk protocol.Packet) error
}

func CreateBasicLoginConn(conn Connection) LoginConnection {
	return &BasicLoginConn{conn: conn}
}

// Basic implementation of LoginConnection
type BasicLoginConn struct {
	conn Connection
}

func (lConn *BasicLoginConn) HS() (protocol.Packet, error) {
	return lConn.readPacket()
}

func (lConn *BasicLoginConn) LoginStart() (protocol.Packet, error) {
	return lConn.readPacket()
}

func (lConn *BasicLoginConn) readPacket() (protocol.Packet, error) {
	pk, err := lConn.conn.ReadPacket()
	if err != nil {
		return protocol.Packet{}, err
	}
	return pk, nil
}

func CreateBasicServerConn(conn Connection, pk protocol.Packet) ServerConnection {
	return &BasicServerConn{conn: conn, statusPK: pk}
}

type BasicServerConn struct {
	conn     Connection
	statusPK protocol.Packet
}

func (sConn *BasicServerConn) Status() (protocol.Packet, error) {
	sConn.conn.WritePacket(sConn.statusPK)
	return sConn.conn.ReadPacket()
}

func (sConn *BasicServerConn) SendPK(pk protocol.Packet) error {
	return sConn.conn.WritePacket(pk)
}
