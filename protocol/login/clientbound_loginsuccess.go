package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundLoginSuccessPacketID byte = 0x02

type ClientBoundLoginSuccess struct {
	UUID     protocol.UUID
	Username protocol.String
}

func (pk ClientBoundLoginSuccess) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundLoginSuccessPacketID,
		pk.UUID,
		pk.Username,
	)
}

func ParseClientBoundLoginSuccess(packet protocol.Packet) (ClientBoundLoginSuccess, error) {
	var pk ClientBoundLoginSuccess

	if packet.ID != ClientBoundLoginSuccessPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.UUID,
		&pk.Username,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
