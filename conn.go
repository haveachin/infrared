package infrared

import (
	"bufio"
	"io"
	"net"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

type PacketWriter interface {
	WritePacket(pk protocol.Packet) error
	WritePackets(pk ...protocol.Packet) error
}

type PacketReader interface {
	ReadPacket() (protocol.Packet, error)
	ReadPackets(n int) ([]protocol.Packet, error)
}

type PacketPeeker interface {
	PeekPacket() (protocol.Packet, error)
	PeekPackets(n int) ([]protocol.Packet, error)
}

type conn struct {
	net.Conn

	r *bufio.Reader
	w io.Writer
}

func newConn(c net.Conn) Conn {
	return &conn{
		Conn: c,
		r:    bufio.NewReader(c),
		w:    c,
	}
}

// Conn is a minecraft Connection
type Conn interface {
	net.Conn
	PacketWriter
	PacketReader
	PacketPeeker

	Reader() *bufio.Reader
}

type Dialer struct {
	net.Dialer
}

func (c *conn) Reader() *bufio.Reader {
	return c.r
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

// ReadPacket read a Packet from Conn.
func (c *conn) ReadPackets(n int) ([]protocol.Packet, error) {
	pks := make([]protocol.Packet, n)
	for i := 0; i < n; i++ {
		pk, err := c.ReadPacket()
		if err != nil {
			return nil, err
		}
		pks[i] = pk
	}
	return pks, nil
}

// PeekPacket peek a Packet from Conn.
func (c *conn) PeekPacket() (protocol.Packet, error) {
	pks, err := c.PeekPackets(1)
	if err != nil {
		return protocol.Packet{}, err
	}

	return pks[0], nil
}

// PeekPackets peeks n Packets from Conn.
func (c *conn) PeekPackets(n int) ([]protocol.Packet, error) {
	return protocol.PeekPackets(c.r, n)
}

//WritePacket write a Packet to Conn.
func (c *conn) WritePacket(pk protocol.Packet) error {
	bb, err := pk.Marshal()
	if err != nil {
		return err
	}
	_, err = c.w.Write(bb)
	return err
}

//WritePackets writes Packets to Conn.
func (c *conn) WritePackets(pks ...protocol.Packet) error {
	for _, pk := range pks {
		if err := c.WritePacket(pk); err != nil {
			return err
		}
	}
	return nil
}

type ProcessingConn struct {
	Conn
	readPks       []protocol.Packet
	handshake     handshaking.ServerBoundHandshake
	remoteAddr    net.Addr
	srvHost       string
	username      string
	proxyProtocol bool
	realIP        bool
	serverIDs     []string
}

func (c ProcessingConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

type ProcessedConn struct {
	ProcessingConn
	ServerConn Conn
}

func (c ProcessedConn) StartPipe() {
	defer c.Close()

	go io.Copy(c.ServerConn, c)
	io.Copy(c, c.ServerConn)
}

func (c ProcessedConn) Close() {
	c.ServerConn.Close()
	c.ProcessingConn.Close()
}
