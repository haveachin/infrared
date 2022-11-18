package java

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/haveachin/infrared/internal/app/infrared"
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
	gatewayID                string
	realIP                   bool
	serverNotFoundMessage    string
	serverNotFoundStatusJSON string

	r *bufio.Reader
	w io.Writer
}

func (c *Conn) Pipe(rc net.Conn) error {
	_, err := io.Copy(rc, c)
	return err
}

func (c Conn) GatewayID() string {
	return c.gatewayID
}

func (c Conn) Edition() infrared.Edition {
	return infrared.JavaEdition
}

func (c *Conn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *Conn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPacket(maxSize int32) (protocol.Packet, error) {
	return protocol.ReadPacket(c.r, maxSize)
}

// ReadPacket read a Packet from Conn.
func (c *Conn) ReadPackets(n int, maxSizes ...int32) ([]protocol.Packet, error) {
	if n != len(maxSizes) {
		return nil, fmt.Errorf("invalid number of max packet sizes: got %d; want %d", len(maxSizes), n)
	}

	pks := make([]protocol.Packet, n)
	for i := 0; i < n; i++ {
		pk, err := c.ReadPacket(maxSizes[i])
		if err != nil {
			return nil, err
		}
		pks[i] = pk
	}
	return pks, nil
}

// PeekPacket peek a Packet from Conn.
func (c *Conn) PeekPacket(maxSize int32) (protocol.Packet, error) {
	pk, err := protocol.PeekPacket(c.r, maxSize)
	if err != nil {
		return protocol.Packet{}, err
	}

	return pk, nil
}

// PeekPackets peeks n Packets from Conn.
func (c *Conn) PeekPackets(n int, maxSizes ...int32) ([]protocol.Packet, error) {
	if n != len(maxSizes) {
		return nil, fmt.Errorf("invalid number of max packet sizes: got %d; want %d", len(maxSizes), n)
	}

	pks := make([]protocol.Packet, n)
	for i := 0; i < n; i++ {
		pk, err := c.PeekPacket(maxSizes[i])
		if err != nil {
			return nil, err
		}
		pks[i] = pk
	}
	return pks, nil
}

// WritePacket write a Packet to Conn.
func (c *Conn) WritePacket(pk protocol.Packet) error {
	bb, err := pk.Marshal()
	if err != nil {
		return err
	}
	_, err = c.w.Write(bb)
	return err
}

// WritePackets writes Packets to Conn.
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
	readPks    []protocol.Packet
	handshake  handshaking.ServerBoundHandshake
	remoteAddr net.Addr
	serverAddr string
	username   string
}

func (pc ProcessedConn) RemoteAddr() net.Addr {
	return pc.remoteAddr
}

func (pc ProcessedConn) Username() string {
	return pc.username
}

func (pc ProcessedConn) ServerAddr() string {
	return pc.serverAddr
}

func (pc ProcessedConn) IsLoginRequest() bool {
	return pc.handshake.IsLoginRequest()
}

func (pc ProcessedConn) DisconnectServerNotFound() error {
	defer pc.Close()

	if pc.handshake.IsLoginRequest() {
		msg := infrared.ExecuteMessageTemplate(pc.serverNotFoundMessage, &pc)
		pk := login.ClientBoundDisconnect{
			Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
		}.Marshal()
		return pc.WritePacket(pk)
	}

	msg := infrared.ExecuteMessageTemplate(pc.serverNotFoundStatusJSON, &pc)
	pk := status.ClientBoundResponse{
		JSONResponse: protocol.String(msg),
	}.Marshal()

	if err := pc.WritePacket(pk); err != nil {
		return err
	}

	ping, err := pc.ReadPacket(status.MaxSizeServerBoundPingRequest)
	if err != nil {
		return err
	}

	return pc.WritePacket(ping)
}
