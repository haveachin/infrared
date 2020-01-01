package net

import (
	"bufio"
	"crypto/cipher"
	"io"
	"net"
	"time"

	"github.com/haveachin/infrared/net/packet"
)

// A Listener is a minecraft Listener
type Listener struct{ net.Listener }

//ListenMC listen as TCP but Accept a net Conn
func ListenMC(addr string) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Listener{l}, nil
}

//Accept a minecraft Conn
func (l Listener) Accept() (Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return Conn{}, err
	}

	return Conn{
		Addr:       conn.RemoteAddr().String(),
		Socket:     conn,
		ByteReader: bufio.NewReader(conn),
		Writer:     conn,
	}, err
}

//Conn is a minecraft Connection
type Conn struct {
	Addr   string
	Socket net.Conn
	io.ByteReader
	io.Writer

	threshold int
}

// DialMC create a Minecraft connection
func DialMC(addr string) (*Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Addr:       conn.RemoteAddr().String(),
		Socket:     conn,
		ByteReader: bufio.NewReader(conn),
		Writer:     conn,
	}, nil
}

// DialMCTimeout acts like DialMC but takes a timeout.
func DialMCTimeout(addr string, timeout time.Duration) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	return &Conn{
		Addr:       conn.RemoteAddr().String(),
		Socket:     conn,
		ByteReader: bufio.NewReader(conn),
		Writer:     conn,
	}, nil
}

// WrapConn warp an net.Conn to MC-Conn
// Helps you modify the connection process (eg. using DialContext).
func WrapConn(conn net.Conn) *Conn {
	return &Conn{
		Addr:       conn.RemoteAddr().String(),
		Socket:     conn,
		ByteReader: bufio.NewReader(conn),
		Writer:     conn,
	}
}

//Close close the connection
func (c *Conn) Close() error { return c.Socket.Close() }

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPacket() (packet.Packet, error) {
	p, err := packet.RecvPacket(c.ByteReader, c.threshold > 0)
	if err != nil {
		return packet.Packet{}, err
	}
	return *p, err
}

//WritePacket write a Packet to Conn.
func (c *Conn) WritePacket(p packet.Packet) error {
	_, err := c.Write(p.Pack(c.threshold))
	return err
}

// SetCipher load the decode/encode stream to this Conn
func (c *Conn) SetCipher(ecoStream, decoStream cipher.Stream) {
	c.ByteReader = bufio.NewReader(cipher.StreamReader{ //Set receiver for AES
		S: decoStream,
		R: c.Socket,
	})
	c.Writer = cipher.StreamWriter{
		S: ecoStream,
		W: c.Socket,
	}
}

// SetThreshold set threshold to Conn.
// The data packet with length longer then threshold
// will be compress when sending.
func (c *Conn) SetThreshold(t int) {
	c.threshold = t
}
