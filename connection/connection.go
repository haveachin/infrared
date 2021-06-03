package connection

import (
	"bufio"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

func ServerAddr(conn HSConnection) string {
	return string(conn.Handshake().ServerAddress)
}

func ServerPort(conn HSConnection) int16 {
	return int16(conn.Handshake().ServerPort)
}

func ProtocolVersion(conn HSConnection) int16 {
	return int16(conn.Handshake().ProtocolVersion)
}

func ParseRequestType(conn HSConnection) RequestType {
	return RequestType(conn.Handshake().NextState)
}

func Pipe(c, s PipeConnection) {
	client := c.getConn()
	server := s.getConn()
	go func() {
		pipe(server, client)
		client.Close()
	}()
	pipe(client, server)
	server.Close()
}

func pipe(c1, c2 ByteConnection) {
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

func CreateBasicPlayerConnection(conn net.Conn, remoteAddr net.Addr) *BasicPlayerConnection {
	c := CreateBasicConnection(conn)
	return &BasicPlayerConnection{
		conn:   c,
		reader: bufio.NewReader(conn),
		addr:   remoteAddr,
	}
}

// Basic implementation of LoginConnection
type BasicPlayerConnection struct {
	conn   ByteConnection
	reader protocol.DecodeReader

	addr net.Addr
	hsPk protocol.Packet
	hs   handshaking.ServerBoundHandshake
}

func (c *BasicPlayerConnection) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
}

func (c *BasicPlayerConnection) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.conn.Write(pk)
	return err
}

func (c *BasicPlayerConnection) Handshake() handshaking.ServerBoundHandshake {
	return c.hs
}

func (c *BasicPlayerConnection) HsPk() protocol.Packet {
	return c.hsPk
}

func (c *BasicPlayerConnection) SetHsPk(pk protocol.Packet) {
	c.hsPk = pk
}

func (c *BasicPlayerConnection) SetHandshake(hs handshaking.ServerBoundHandshake) {
	c.hs = hs
}

func (c *BasicPlayerConnection) RemoteAddr() net.Addr {
	return c.addr
}

func (c *BasicPlayerConnection) getConn() ByteConnection {
	return c.conn
}

func CreateBasicServerConn(c net.Conn) ServerConnection {
	conn := CreateBasicConnection(c)
	return &BasicServerConn{conn: conn, reader: bufio.NewReader(conn)}
}

type BasicServerConn struct {
	conn   ByteConnection
	reader protocol.DecodeReader
}

func (c *BasicServerConn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
}

func (c *BasicServerConn) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.conn.Write(pk)
	return err
}

func (c *BasicServerConn) getConn() ByteConnection {
	return c.conn
}

func CreateBasicConnection(conn net.Conn) *BasicConnection {
	return &BasicConnection{connection: conn}
}

type BasicConnection struct {
	connection ByteConnection
}

func (c *BasicConnection) Read(b []byte) (n int, err error) {
	return c.connection.Read(b)
}

func (c *BasicConnection) Write(b []byte) (n int, err error) {
	return c.connection.Write(b)
}

func (c *BasicConnection) Close() error {
	return c.connection.Close()
}
