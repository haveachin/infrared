package connection

import (
	"bufio"
	"net"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

type RequestType int8

const (
	UnknownRequest RequestType = 0
	StatusRequest  RequestType = 1
	LoginRequest   RequestType = 2
)

type Connection interface {
	infrared.PacketWriter
	infrared.PacketReader
}

type HSConnection interface {
	Hs() (handshaking.ServerBoundHandshake, bool)
	HsPk() (protocol.Packet, error)
}

type PlayerConnection interface {
	HSConnection
	RequestType() RequestType
}

type LoginConnection interface {
	HSConnection
	LoginStart() (protocol.Packet, error) // Need more work
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

func (conn *BasicLoginConn) Hs() (handshaking.ServerBoundHandshake, bool) {
	return handshaking.ServerBoundHandshake{}, false
}

func (lConn *BasicLoginConn) HsPk() (protocol.Packet, error) {
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

func CreateBasicConnection(conn net.Conn) *BasicConnection {
	return &BasicConnection{conn: conn, reader: bufio.NewReader(conn)}
}

type BasicConnection struct {
	conn   net.Conn
	reader protocol.DecodeReader
}

func (c *BasicConnection) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test
	_, err := c.conn.Write(pk)
	return err
}

func (c *BasicConnection) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
}
