package connection

import (
	"bufio"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/login"
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
	//use this as blocking method so that when client returns EOF the code will continue to run code
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

func CreateBasicPlayerConnection2(conn net.Conn, remoteAddr net.Addr) *BasicPlayerConnection {
	c := CreateBasicConnection2(conn, remoteAddr)
	return &BasicPlayerConnection{conn: c}
}

// Basic implementation of LoginConnection
type BasicPlayerConnection struct {
	conn Connection

	addr    net.Addr
	hsPk    protocol.Packet
	loginPk protocol.Packet
	hs      handshaking.ServerBoundHandshake
	mcName  string

	hasHS    bool
	hasHSPk  bool
	hasLogin bool
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

	pk, _ := c.HsPk()
	var err error
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
	if c.mcName != "" {
		return c.mcName, nil
	}
	p, _ := c.LoginStart()
	pk, err := login.UnmarshalServerBoundLoginStart(p)
	if err != nil {
		return "", err
	}
	c.mcName = string(pk.Name)
	return c.mcName, nil
}

func (c *BasicPlayerConnection) LoginStart() (protocol.Packet, error) {
	pk, _ := c.ReadPacket() //Need tests for error handling
	c.loginPk = pk
	c.hasLogin = true
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

func CreateBasicServerConn2(c net.Conn) ServerConnection {
	conn := CreateBasicConnection(c)
	return &BasicServerConn{conn: conn}
}

type BasicServerConn struct {
	conn     Connection
	statusPK protocol.Packet
}

func (c *BasicServerConn) Status(pk protocol.Packet) (protocol.Packet, error) {
	c.conn.WritePacket(pk)
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

func CreateBasicConnection2(conn net.Conn, addr net.Addr) *BasicConnection {
	return &BasicConnection{
		connection: conn,
		reader:     bufio.NewReader(conn),
		remoteAddr: addr,
	}
}

type BasicConnection struct {
	connection net.Conn
	reader     protocol.DecodeReader
	remoteAddr net.Addr
}

func (c *BasicConnection) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.Write(pk)
	return err
}

func (c *BasicConnection) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
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
