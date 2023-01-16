package play

import "github.com/haveachin/infrared/internal/pkg/java/protocol"

const IDClientBoundDisconnect byte = 0x17

type ClientBoundDisconnect struct {
	Reason protocol.Chat
}

func (pk ClientBoundDisconnect) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		IDClientBoundDisconnect,
		pk.Reason,
	)
}
