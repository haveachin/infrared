package login

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const ClientBoundDisconnectID int32 = 0x00

type ClientBoundDisconnect struct {
	Reason protocol.Chat
}

func (pk ClientBoundDisconnect) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ClientBoundDisconnectID,
		pk.Reason,
	)
}
