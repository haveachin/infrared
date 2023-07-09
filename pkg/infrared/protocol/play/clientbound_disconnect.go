package play

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const IDClientBoundDisconnect int32 = 0x17

type ClientBoundDisconnect struct {
	Reason protocol.Chat
}

func (pk ClientBoundDisconnect) Marshal(packet *protocol.Packet) {
	packet.Encode(
		IDClientBoundDisconnect,
		pk.Reason,
	)
}
