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
