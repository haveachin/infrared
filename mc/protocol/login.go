package protocol

import pk "github.com/haveachin/infrared/mc/packet"

const (
	ClientLoginStartPacketID      = 0x00
	ServerLoginDisconnectPacketID = 0x00
	ServerLoginSetCompressionPacketID = 0x03
)

type ClientLoginStart struct {
	Name pk.String
}

func (packet ClientLoginStart) ID() byte {
	return ClientLoginStartPacketID
}

func ParseClientLoginStart(packet pk.Packet) (ClientLoginStart, error) {
	var start ClientLoginStart

	if packet.ID != ClientLoginStartPacketID {
		return start, ErrInvalidPacketID
	}

	if err := packet.Scan(&start.Name); err != nil {
		return start, err
	}

	return start, nil
}

type ServerLoginSetCompression struct {
	Threshold pk.VarInt
}

func (packet ServerLoginSetCompression) ID() byte {
	return ServerLoginSetCompressionPacketID
}

type LoginDisconnect struct {
	Reason pk.Chat
}

func (packet LoginDisconnect) Marshal() pk.Packet {
	return pk.Marshal(ServerLoginDisconnectPacketID, packet.Reason)
}
