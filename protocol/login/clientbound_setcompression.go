package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundSetCompressionPacketID byte = 0x03

type ClientBoundSetCompression struct {
	Threshold protocol.VarInt
}

func (pk ClientBoundSetCompression) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundSetCompressionPacketID,
		pk.Threshold,
	)
}

func ParseClientBoundSetCompression(packet protocol.Packet) (ClientBoundSetCompression, error) {
	var pk ClientBoundSetCompression

	if packet.ID != ClientBoundSetCompressionPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(&pk.Threshold); err != nil {
		return pk, err
	}

	return pk, nil
}
