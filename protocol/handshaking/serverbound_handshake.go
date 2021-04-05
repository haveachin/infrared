package handshaking

import (
	"github.com/haveachin/infrared/protocol"
	"strings"
)

const (
	ServerBoundHandshakePacketID byte = 0x00

	ServerBoundHandshakeStatusState = protocol.Byte(1)
	ServerBoundHandshakeLoginState  = protocol.Byte(2)

	ForgeAddressSuffix  = "\x00FML\x00"
	Forge2AddressSuffix = "\x00FML2\x00"
)

type ServerBoundHandshake struct {
	ProtocolVersion protocol.VarInt
	ServerAddress   protocol.String
	ServerPort      protocol.UnsignedShort
	NextState       protocol.Byte
}

func (pk ServerBoundHandshake) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ServerBoundHandshakePacketID,
		pk.ProtocolVersion,
		pk.ServerAddress,
		pk.ServerPort,
		pk.NextState,
	)
}

func UnmarshalServerBoundHandshake(packet protocol.Packet) (ServerBoundHandshake, error) {
	var pk ServerBoundHandshake

	if packet.ID != ServerBoundHandshakePacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.ProtocolVersion,
		&pk.ServerAddress,
		&pk.ServerPort,
		&pk.NextState,
	); err != nil {
		return pk, err
	}

	return pk, nil
}

func (pk ServerBoundHandshake) IsStatusRequest() bool {
	return pk.NextState == ServerBoundHandshakeStatusState
}

func (pk ServerBoundHandshake) IsLoginRequest() bool {
	return pk.NextState == ServerBoundHandshakeLoginState
}

func (pk ServerBoundHandshake) IsForgeAddress() bool {
	addr := string(pk.ServerAddress)

	if strings.HasSuffix(addr, ForgeAddressSuffix) {
		return true
	}

	if strings.HasSuffix(addr, Forge2AddressSuffix) {
		return true
	}

	return false
}

func (pk ServerBoundHandshake) ParseServerAddress() string {
	addr := string(pk.ServerAddress)
	addr = strings.TrimSuffix(addr, ForgeAddressSuffix)
	addr = strings.TrimSuffix(addr, Forge2AddressSuffix)
	addr = strings.Trim(addr, ".")
	return addr
}
