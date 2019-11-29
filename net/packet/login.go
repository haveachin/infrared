package packet

const (
	LoginStartPacketID      = 0x00
	LoginDisconnectPacketID = 0x00
)

type LoginStart struct {
	Name String
}

func ParseLoginStart(pk Packet) (LoginStart, error) {
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
	Reason Chat
}

func (pk LoginDisconnect) Marshal() Packet {
	return Marshal(LoginDisconnectPacketID, pk.Reason)
}
