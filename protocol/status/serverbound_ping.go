package status

import (
	"github.com/haveachin/infrared/protocol"
)

const ServerBoundPingPacketID byte = 0x01

type ServerBoundPing struct {
	Payload protocol.Long
}

func (pk ServerBoundPing) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ServerBoundPingPacketID,
		pk.Payload,
	)
}

func UnmarshalServerBoundPing(packet protocol.Packet) (ServerBoundPing, error) {
	var pk ServerBoundPing

	if packet.ID != ServerBoundPingPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.Payload,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
