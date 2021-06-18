package connection

import (
	"bufio"
	"errors"
	"io"
	"net"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrNoNameYet = errors.New("we dont have the name of this player yet")
)

type HandshakeChannel chan<- HandshakeConn

type NewServerConnFactory func(timeout time.Duration) (ServerConnFactory, error)

type ServerConnFactory func(string) (ServerConn, error)
type HandshakeConnFactory func(Conn, net.Addr) (HandshakeConn, error)

type RequestType int8

const (
	UnknownRequest RequestType = 0
	StatusRequest  RequestType = 1
	LoginRequest   RequestType = 2
)

type PipeConn interface {
	conn() net.Conn
}

type Conn interface {
	WritePacket(p protocol.Packet) error
	ReadPacket() (protocol.Packet, error)
}

func ServerAddr(conn HandshakeConn) string {
	return string(conn.Handshake.ServerAddress)
}

func ServerPort(conn HandshakeConn) int16 {
	return int16(conn.Handshake.ServerPort)
}

func ProtocolVersion(conn HandshakeConn) int16 {
	return int16(conn.Handshake.ProtocolVersion)
}

func ParseRequestType(conn HandshakeConn) RequestType {
	return RequestType(conn.Handshake.NextState)
}

func Pipe(c, s PipeConn) {
	client := c.conn()
	server := s.conn()
	go func() {
		io.Copy(server, client)
		client.Close()
	}()
	io.Copy(client, server)
	server.Close()
}

func pipe(c1, c2 net.Conn) {
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

func NewHandshakeConn(conn net.Conn, remoteAddr net.Addr) HandshakeConn {
	return HandshakeConn{
		netConn: conn,
		reader:  bufio.NewReader(conn),
		addr:    remoteAddr,
	}
}

type HandshakeConn struct {
	netConn net.Conn
	reader  protocol.DecodeReader
	addr    net.Addr

	Handshake       handshaking.ServerBoundHandshake
	HandshakePacket protocol.Packet
}

func (hsConn HandshakeConn) RemoteAddr() net.Addr {
	return hsConn.addr
}

func (conn HandshakeConn) conn() net.Conn {
	return conn.netConn
}

func (conn HandshakeConn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(conn.reader)
}

func (conn HandshakeConn) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := conn.netConn.Write(pk)
	return err
}

func NewServerConn(conn net.Conn) ServerConn {
	return ServerConn{netConn: conn, reader: bufio.NewReader(conn)}
}

type ServerConn struct {
	netConn net.Conn
	reader  protocol.DecodeReader
}

func (c ServerConn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.reader)
}

func (c ServerConn) WritePacket(p protocol.Packet) error {
	pk, _ := p.Marshal() // Need test for err part of this line
	_, err := c.netConn.Write(pk)
	return err
}

func (c ServerConn) conn() net.Conn {
	return c.netConn
}
