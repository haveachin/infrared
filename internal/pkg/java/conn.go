package java

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
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

type Conn struct {
	net.Conn
	gatewayID     string
	proxyProtocol bool
	realIP        bool

	r *bufio.Reader
	w io.Writer
}

func (c *Conn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *Conn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPacket() (protocol.Packet, error) {
	return protocol.ReadPacket(c.r)
}

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPackets(n int) ([]protocol.Packet, error) {
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
func (c *Conn) PeekPacket() (protocol.Packet, error) {
	pks, err := c.PeekPackets(1)
	if err != nil {
		return protocol.Packet{}, err
	}

	return pks[0], nil
}

// PeekPackets peeks n Packets from Conn.
func (c *Conn) PeekPackets(n int) ([]protocol.Packet, error) {
	return protocol.PeekPackets(c.r, n)
}

//WritePacket write a Packet to Conn.
func (c *Conn) WritePacket(pk protocol.Packet) error {
	bb, err := pk.Marshal()
	if err != nil {
		return err
	}
	_, err = c.w.Write(bb)
	return err
}

//WritePackets writes Packets to Conn.
func (c *Conn) WritePackets(pks ...protocol.Packet) error {
	for _, pk := range pks {
		if err := c.WritePacket(pk); err != nil {
			return err
		}
	}
	return nil
}

type ProcessedConn struct {
	Conn
	readPks       []protocol.Packet
	handshake     handshaking.ServerBoundHandshake
	remoteAddr    net.Addr
	serverAddr    string
	username      string
	proxyProtocol bool
	realIP        bool
}

func (c ProcessedConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c ProcessedConn) GatewayID() string {
	return c.gatewayID
}

func (c ProcessedConn) Username() string {
	return c.username
}

func (c ProcessedConn) ServerAddr() string {
	return c.serverAddr
}

func (c ProcessedConn) Disconnect(msg string) error {
	defer c.Close()

	var pk protocol.Packet
	if c.handshake.IsLoginRequest() {
		pk = login.ClientBoundDisconnect{
			Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
		}.Marshal()
	} else {
		pk = status.ClientBoundResponse{
			JSONResponse: protocol.String(msg),
		}.Marshal()
	}
	return c.WritePacket(pk)
}
