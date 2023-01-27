package java

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"net"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/cfb8"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
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
	gatewayID                  string
	realIP                     bool
	serverNotFoundDisconnector PlayerDisconnecter

	isEncrypted bool
	r           *bufio.Reader
	w           io.Writer
}

func (c *Conn) Pipe(rc net.Conn) (int64, error) {
	return io.Copy(rc, c)
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
	bb := pk.Marshal()
	_, err := c.w.Write(bb)
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

// SetCipher sets the decode/encode stream for this Conn
func (c *Conn) SetCipher(ecoStream, decoStream cipher.Stream) {
	c.isEncrypted = true
	c.r = bufio.NewReader(cipher.StreamReader{
		S: decoStream,
		R: c.Conn,
	})
	c.w = cipher.StreamWriter{
		S: ecoStream,
		W: c.Conn,
	}
}

func (c *Conn) SetEncryption(sharedSecret []byte) error {
	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return err
	}

	c.SetCipher(
		cfb8.NewEncrypter(block, sharedSecret),
		cfb8.NewDecrypter(block, sharedSecret),
	)
	return nil
}

func (c *Conn) ForceClose() error {
	if err := c.Conn.(*net.TCPConn).SetLinger(0); err != nil {
		return err
	}
	return c.Conn.Close()
}

type Player struct {
	Conn
	readPks    []protocol.Packet
	handshake  handshaking.ServerBoundHandshake
	remoteAddr net.Addr
	serverAddr string
	username   string
}

func (p Player) Version() infrared.Version {
	return protocol.Version(p.handshake.ProtocolVersion)
}

func (p Player) RemoteAddr() net.Addr {
	return p.remoteAddr
}

func (p Player) Username() string {
	return p.username
}

func (p Player) ServerAddr() string {
	return p.serverAddr
}

func (p Player) IsLoginRequest() bool {
	return p.handshake.IsLoginRequest()
}

func (p *Player) DisconnectServerNotFound() error {
	return p.serverNotFoundDisconnector.DisconnectPlayer(p, infrared.ApplyTemplates(
		infrared.TimeMessageTemplates(),
		infrared.PlayerMessageTemplates(p),
	))
}

func (p *Player) RemoteIP() net.IP {
	return p.RemoteAddr().(*net.TCPAddr).IP
}
