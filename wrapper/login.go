package wrapper

import "github.com/Tnze/go-mc/net/packet"

const (
	LoginStartPacketID      = 0x00
	LoginDisconnectPacketID = 0x00
)

type LoginStart struct {
	Name packet.String
}

func ParseLoginStart(pk packet.Packet) (LoginStart, error) {
	var start LoginStart

	if pk.ID != LoginStartPacketID {
		return start, ErrInvalidPacketID
	}

	if err := pk.Scan(&start.Name); err != nil {
		return start, err
	}

	return start, nil
}

type LoginDisconnect struct {
	Reason packet.Chat
}

func (pk LoginDisconnect) Marshal() packet.Packet {
	return packet.Marshal(LoginDisconnectPacketID, pk.Reason)
}
