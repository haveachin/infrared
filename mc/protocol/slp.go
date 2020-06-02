package protocol

import (
	pk "github.com/haveachin/infrared/mc/packet"
	"strings"
)

const (
	SLPHandshakePacketID byte = 0x00
	SLPRequestPacketID   byte = 0x00
	SLPResponsePacketID  byte = 0x00
	SLPPingPacketID      byte = 0x01
	SLPPongPacketID      byte = 0x01

	SLPHandshakeStatusState = pk.Byte(1)
	SLPHandshakeLoginState  = pk.Byte(2)

	ForgeAddressSuffix  = "\x00FML\x00"
	Forge2AddressSuffix = "\x00FML2\x00"
)

type SLPHandshake struct {
	ProtocolVersion pk.VarInt
	ServerAddress   pk.String
	ServerPort      pk.UnsignedShort
	NextState       pk.Byte
}

func ParseSLPHandshake(packet pk.Packet) (SLPHandshake, error) {
	var handshake SLPHandshake

	if packet.ID != SLPHandshakePacketID {
		return handshake, ErrInvalidPacketID
	}

	if err := packet.Scan(
		&handshake.ProtocolVersion,
		&handshake.ServerAddress,
		&handshake.ServerPort,
		&handshake.NextState); err != nil {
		return handshake, err
	}

	return handshake, nil
}

func (handshake SLPHandshake) Marshal() pk.Packet {
	return pk.Marshal(
		SLPHandshakePacketID,
		handshake.ProtocolVersion,
		handshake.ServerAddress,
		handshake.ServerPort,
		handshake.NextState)
}

func (handshake SLPHandshake) IsStatusRequest() bool {
	return handshake.NextState == SLPHandshakeStatusState
}

func (handshake SLPHandshake) IsLoginRequest() bool {
	return handshake.NextState == SLPHandshakeLoginState
}

func (handshake SLPHandshake) IsForgeAddress() bool {
	addr := string(handshake.ServerAddress)

	if strings.HasSuffix(addr, ForgeAddressSuffix) {
		return true
	}

	if strings.HasSuffix(addr, Forge2AddressSuffix) {
		return true
	}

	return false
}

func (handshake SLPHandshake) ParseServerAddress() string {
	addr := string(handshake.ServerAddress)
	addr = strings.TrimSuffix(addr, ForgeAddressSuffix)
	addr = strings.TrimSuffix(addr, Forge2AddressSuffix)
	addr = strings.Trim(addr, ".")
	return addr
}

type SLPResponse struct {
	JSONResponse pk.String
}

func (packet SLPResponse) Marshal() pk.Packet {
	return pk.Marshal(SLPResponsePacketID, packet.JSONResponse)
}
