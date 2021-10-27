package infrared

import (
	"bufio"
	"crypto/cipher"
	"github.com/haveachin/infrared/protocol"
	"io"
	"net"
)

type PacketWriter interface {
	WritePacket(p protocol.Packet) error
}

type PacketReader interface {
	ReadPacket() (protocol.Packet, error)
}

type PacketPeeker interface {
	PeekPacket() (protocol.Packet, error)
}

type conn struct {
	net.Conn

	r *bufio.Reader
	w io.Writer
}

type Listener struct {
	net.Listener
}

func Listen(addr string) (Listener, error) {
	l, err := net.Listen("tcp", addr)
	return Listener{Listener: l}, err
}

func (l Listener) Accept() (Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return wrapConn(conn), nil
}

// Conn is a minecraft Connection
type Conn interface {
	net.Conn
	PacketWriter
	PacketReader
	PacketPeeker

	Reader() *bufio.Reader
}

// wrapConn warp an net.Conn to infared.conn
func wrapConn(c net.Conn) *conn {
	return &conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}
}

type Dialer struct {
	net.Dialer
}

// Dial create a Minecraft connection
func (d Dialer) Dial(addr string) (Conn, error) {
	conn, err := d.Dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return wrapConn(conn), nil
}

func (c *conn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

// ReadPacket read a Packet from Conn.
func (c *conn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.r)
}

// PeekPacket peeks a Packet from Conn.
func (c *conn) PeekPacket() (protocol.Packet, error) {
	return protocol.PeekPacket(c.r)
}

//WritePacket write a Packet to Conn.
func (c *conn) WritePacket(p protocol.Packet) error {
	pk, err := p.Marshal()
	if err != nil {
		return err
	}
	_, err = c.w.Write(pk)
	return err
}

// SetCipher sets the decode/encode stream for this Conn
func (c *conn) SetCipher(ecoStream, decoStream cipher.Stream) {
	c.r = bufio.NewReader(cipher.StreamReader{
		S: decoStream,
		R: c.Conn,
	})
	c.w = cipher.StreamWriter{
		S: ecoStream,
		W: c.Conn,
	}
}

func (c *conn) Reader() *bufio.Reader {
	return c.r
}
