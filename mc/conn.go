package mc

import (
	"bufio"
	"crypto/cipher"
	"io"
	"net"
	"time"

	pk "github.com/haveachin/infrared/mc/packet"
)

// A Listener is a minecraft Listener
type Listener struct{ net.Listener }

//Conn is a minecraft Connection
type Conn struct {
	net.Conn

	r         *bufio.Reader
	w         io.Writer
	threshold int
}

//Listen listen as TCP but Accept a mc Conn
func Listen(addr string) (*Listener, error) {
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

	return WrapConn(conn), nil
}

// Dial create a Minecraft connection
func Dial(addr string) (Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return Conn{}, err
	}

	return WrapConn(conn), nil
}

// DialTimeout acts like DialMC but takes a timeout.
func DialTimeout(addr string, timeout time.Duration) (Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return Conn{}, err
	}

	return WrapConn(conn), nil
}

// WrapConn warp an net.Conn to MC-Conn
// Helps you modify the connection process (eg. using DialContext).
func WrapConn(conn net.Conn) Conn {
	return Conn{
		Conn:      conn,
		r:         bufio.NewReader(conn),
		w:         conn,
		threshold: 0,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *Conn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPacket() (pk.Packet, error) {
	return pk.Read(c.r, c.threshold > 0)
}

// PeekPacket peeks a Packet from Conn.
func (c *Conn) PeekPacket() (pk.Packet, error) {
	return pk.Peek(c.r, c.threshold > 0)
}

//WritePacket write a Packet to Conn.
func (c *Conn) WritePacket(p pk.Packet) error {
	_, err := c.w.Write(p.Pack(c.threshold))
	return err
}

// SetCipher load the decode/encode stream to this Conn
func (c *Conn) SetCipher(ecoStream, decoStream cipher.Stream) {
	c.r = bufio.NewReader(cipher.StreamReader{
		S: decoStream,
		R: c.Conn,
	})
	c.w = cipher.StreamWriter{
		S: ecoStream,
		W: c.Conn,
	}
}

// SetThreshold set threshold to Conn.
// The data packet with length longer then threshold
// will be compress when sending.
func (c *Conn) SetThreshold(t int) {
	c.threshold = t
}
