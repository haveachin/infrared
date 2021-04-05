package status

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundPongPacketID byte = 0x01

type ClientBoundPong struct {
	Payload protocol.Long
}

func (pk ClientBoundPong) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundPongPacketID,
		pk.Payload,
	)
}

func UnmarshalClientBoundPong(packet protocol.Packet) (ClientBoundPong, error) {
	var pk ClientBoundPong

	if packet.ID != ClientBoundPongPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.Payload,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
