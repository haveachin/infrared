package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundDisconnectPacketID byte = 0x00

type ClientBoundDisconnect struct {
	Reason protocol.Chat
}

func (pk ClientBoundDisconnect) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundDisconnectPacketID,
		pk.Reason,
	)
}

func UnmarshalClientBoundDisconnect(packet protocol.Packet) (ClientBoundDisconnect, error) {
	var pk ClientBoundDisconnect

	if packet.ID != ClientBoundDisconnectPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.Reason,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
