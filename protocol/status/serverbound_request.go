package status

import (
	"github.com/haveachin/infrared/protocol"
)

const ServerBoundRequestPacketID byte = 0x00

type ServerBoundRequest struct{}

func (pk ServerBoundRequest) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ServerBoundRequestPacketID,
	)
}

func UnmarshalServerBoundRequest(packet protocol.Packet) (ServerBoundRequest, error) {
	var pk ServerBoundRequest

	if packet.ID != ServerBoundRequestPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	return pk, nil
}
