package play

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const ClientBoundDisconnectID int32 = 0x17

type ClientBoundDisconnect struct {
	Reason protocol.Chat
}

func (pk ClientBoundDisconnect) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ClientBoundDisconnectID,
		pk.Reason,
	)
}
