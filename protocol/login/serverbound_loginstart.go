package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ServerBoundLoginStartPacketID byte = 0x00

type ServerLoginStart struct {
	Name protocol.String
}

func UnmarshalServerBoundLoginStart(packet protocol.Packet) (ServerLoginStart, error) {
	var pk ServerLoginStart

	if packet.ID != ServerBoundLoginStartPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(&pk.Name); err != nil {
		return pk, err
	}

	return pk, nil
}
