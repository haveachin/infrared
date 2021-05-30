package connection

import (
	"bufio"
	"fmt"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

func ServerAddr(conn HSConnection) string {
	hs, _ := conn.Handshake()
	return string(hs.ServerAddress)
}

func ServerPort(conn HSConnection) int16 {
	hs, _ := conn.Handshake()
	return int16(hs.ServerPort)
}

func ProtocolVersion(conn HSConnection) int16 {
	hs, _ := conn.Handshake()
	return int16(hs.ProtocolVersion)
}

func ParseRequestType(conn HSConnection) RequestType {
	hs, _ := conn.Handshake()
	return RequestType(hs.NextState)
}

func Pipe(client, server PipeConnection) {
	go pipe(server, client)
	pipe(client, server)
}

func pipe(c1, c2 PipeConnection) {
	buffer := make([]byte, 0xffff)

	for {
		n, err := c1.Read(buffer)
		if err != nil {
			return
		}

		data := buffer[:n]

		_, err = c2.Write(data)
		if err != nil {
			return
		}
	}
}

func CreateBasicPlayerConnection(conn Connection, remoteAddr net.Addr) *BasicPlayerConnection {
	return &BasicPlayerConnection{conn: conn, addr: remoteAddr}
}

// Basic implementation of LoginConnection
type BasicPlayerConnection struct {
	conn Connection

	addr    net.Addr
	hsPk    protocol.Packet
	loginPk protocol.Packet
	hs      handshaking.ServerBoundHandshake

	hasHS   bool
	hasHSPk bool
}

func (c *BasicPlayerConnection) ServerAddr() string {
	hs, _ := c.Handshake()
	return string(hs.ServerAddress)
}

func (c *BasicPlayerConnection) ReadPacket() (protocol.Packet, error) {
	return c.conn.ReadPacket()
}

func (c *BasicPlayerConnection) WritePacket(p protocol.Packet) error {
	return c.conn.WritePacket(p)
}

func (c *BasicPlayerConnection) RemoteAddr() net.Addr {
	return c.addr
}

func (c *BasicPlayerConnection) Handshake() (handshaking.ServerBoundHandshake, error) {
	if c.hasHS {
		return c.hs, nil
	}

	pk, err := c.HsPk()
	if err != nil {
		return c.hs, err
	}
	c.hs, err = handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return c.hs, err
	}
	c.hasHS = true
	return c.hs, nil
}

func (c *BasicPlayerConnection) HsPk() (protocol.Packet, error) {
	if c.hasHSPk {
		return c.hsPk, nil
	}
	pk, err := c.ReadPacket()
	if err != nil {
		return pk, ErrCantGetHSPacket
	}
	c.hsPk = pk
	c.hasHSPk = true
	return pk, nil
}

func (c *BasicPlayerConnection) Name() (string, error) {
	return "", ErrNoNameYet
}

func (c *BasicPlayerConnection) LoginStart() (protocol.Packet, error) {
	pk, _ := c.ReadPacket()
	c.loginPk = pk
	return pk, nil
}

func (c *BasicPlayerConnection) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *BasicPlayerConnection) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func CreateBasicServerConn(conn Connection, pk protocol.Packet) ServerConnection {
	return &BasicServerConn{conn: conn, statusPK: pk}
}

type BasicServerConn struct {
	conn     Connection
	statusPK protocol.Packet
}

func (c *BasicServerConn) Status(pk protocol.Packet) (protocol.Packet, error) {
	c.conn.WritePacket(c.statusPK)
	return c.conn.ReadPacket()
}

func (c *BasicServerConn) SendPK(pk protocol.Packet) error {
	return c.conn.WritePacket(pk)
}

func (c *BasicServerConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *BasicServerConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func CreateBasicConnection(conn net.Conn) *BasicConnection {
	return &BasicConnection{connection: conn, reader: bufio.NewReader(conn)}
}

type BasicConnection struct {
	connection net.Conn
	reader     protocol.DecodeReader
	remoteAddr net.Addr
}

func (c *BasicConnection) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.Write(pk)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func (c *BasicConnection) ReadPacket() (protocol.Packet, error) {
	pk, err := protocol.ReadPacket(c.reader)
	if err != nil {
		fmt.Println(err)
	}
	return pk, err
}

func (c *BasicConnection) Read(b []byte) (n int, err error) {
	return c.connection.Read(b)
}

func (c *BasicConnection) Write(b []byte) (n int, err error) {
	return c.connection.Write(b)
}

func (c *BasicConnection) RemoteAddr() net.Addr {
	return c.remoteAddr
}
