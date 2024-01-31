package status

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const IDServerBoundRequest int32 = 0x00

type ServerBoundRequest struct{}

func (pk ServerBoundRequest) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		IDServerBoundRequest,
	)
}
