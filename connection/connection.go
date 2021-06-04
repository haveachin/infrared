package connection

import (
	"bufio"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

func ServerAddr(conn HandshakeConn) string {
	return string(conn.Handshake().ServerAddress)
}

func ServerPort(conn HandshakeConn) int16 {
	return int16(conn.Handshake().ServerPort)
}

func ProtocolVersion(conn HandshakeConn) int16 {
	return int16(conn.Handshake().ProtocolVersion)
}

func ParseRequestType(conn HandshakeConn) RequestType {
	return RequestType(conn.Handshake().NextState)
}

func Pipe(c, s PipeConn) {
	client := c.conn()
	server := s.conn()
	go func() {
		pipe(server, client)
		client.Close()
	}()
	pipe(client, server)
	server.Close()
}

func pipe(c1, c2 ByteConn) {
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

func NewBasicPlayerConn(conn net.Conn, remoteAddr net.Addr) *BasicPlayerConn {
	c := NewBasicConn(conn)
	return &BasicPlayerConn{
		byteConn:  c,
		reader: bufio.NewReader(conn),
		addr:   remoteAddr,
	}
}

// Basic implementation of LoginConnection
type BasicPlayerConn struct {
	byteConn  ByteConn
	reader protocol.DecodeReader

	addr net.Addr
	hsPk protocol.Packet
	hs   handshaking.ServerBoundHandshake
}

func (conn *BasicPlayerConn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(conn.reader)
}

func (conn *BasicPlayerConn) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := conn.byteConn.Write(pk)
	return err
}

func (conn *BasicPlayerConn) Handshake() handshaking.ServerBoundHandshake {
	return conn.hs
}

func (conn *BasicPlayerConn) HandshakePacket() protocol.Packet {
	return conn.hsPk
}

func (conn *BasicPlayerConn) SetHandshakePacket(pk protocol.Packet) {
	conn.hsPk = pk
}

func (conn *BasicPlayerConn) SetHandshake(hs handshaking.ServerBoundHandshake) {
	conn.hs = hs
}

func (conn *BasicPlayerConn) RemoteAddr() net.Addr {
	return conn.addr
}

func (conn *BasicPlayerConn) conn() ByteConn {
	return conn.byteConn
}

func NewBasicServerConn(c net.Conn) *BasicServerConn {
	conn := NewBasicConn(c)
	return &BasicServerConn{byteConn: conn, reader: bufio.NewReader(conn)}
}

type BasicServerConn struct {
	byteConn  ByteConn
	reader protocol.DecodeReader
}

func (c *BasicServerConn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
}

func (c *BasicServerConn) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.byteConn.Write(pk)
	return err
}

func (c *BasicServerConn) conn() ByteConn {
	return c.byteConn
}

func NewBasicConn(conn net.Conn) *BasicConn {
	return &BasicConn{netConn: conn}
}

type BasicConn struct {
	netConn net.Conn
}

func (conn *BasicConn) Read(b []byte) (n int, err error) {
	return conn.netConn.Read(b)
}

func (conn *BasicConn) Write(b []byte) (n int, err error) {
	return conn.netConn.Write(b)
}

func (conn *BasicConn) Close() error {
	return conn.netConn.Close()
}
