package bedrock

import (
	"bytes"
	"net"

	"github.com/haveachin/infrared/internal"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
	"github.com/sandertv/go-raknet"
)

// Conn is a minecraft Connection
type Conn struct {
	*raknet.Conn

	decoder *packet.Decoder
	encoder *packet.Encoder

	gatewayID                  string
	proxyProtocol              bool
	serverNotFoundDisconnector PlayerDisconnecter
	compression                packet.Compression
}

func (c *Conn) EnableCompression(compression packet.Compression) {
	c.decoder.EnableCompression(compression)
	c.encoder.EnableCompression(compression)
}

func (c *Conn) ReadPackets() ([]packet.Data, error) {
	pks, err := c.decoder.Decode()
	if err != nil {
		return nil, err
	}

	pksData := make([]packet.Data, 0, len(pks))
	for _, pk := range pks {
		pkData, err := packet.ParseData(pk)
		if err != nil {
			return nil, err
		}

		pksData = append(pksData, pkData)
	}
	return pksData, nil
}

func (c *Conn) WritePacket(pk packet.Packet) error {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()
	pkHeader := packet.Header{
		PacketID: pk.ID(),
	}
	pkHeader.Write(buf)
	pk.Marshal(protocol.NewWriter(buf))
	return c.encoder.Encode(buf.Bytes())
}

func (c *Conn) Pipe(rc net.Conn) (int64, error) {
	var nn int64
	for {
		pk, err := c.Conn.ReadPacket()
		if err != nil {
			return nn, err
		}

		n, err := rc.Write(pk)
		if err != nil {
			return nn, err
		}
		nn += int64(n)
	}
}

func (c *Conn) GatewayID() string {
	return c.gatewayID
}

func (c *Conn) Edition() infrared.Edition {
	return infrared.BedrockEdition
}

type Player struct {
	Conn
	remoteAddr    net.Addr
	serverAddr    string
	username      string
	proxyProtocol bool
	version       int32

	requestNetworkSettingsPkData *packet.Data
	loginPkData                  packet.Data
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
	return true
}

func (p *Player) DisconnectServerNotFound() error {
	return p.serverNotFoundDisconnector.DisconnectPlayer(p, infrared.ApplyTemplates(
		infrared.TimeMessageTemplates(),
		infrared.PlayerMessageTemplates(p),
	))
}

func (p *Player) RemoteIP() net.IP {
	return p.RemoteAddr().(*net.UDPAddr).IP
}
