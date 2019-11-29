package packet

import "errors"

const HandshakePacketID = 0x00

type Handshake struct {
	ProtocolVersion VarInt
	ServerAddress   String
	ServerPort      UnsignedShort
	NextState       Byte
}

func ParseHandshake(pk Packet) (Handshake, error) {
	var handshake Handshake

	if pk.ID != HandshakePacketID {
		return handshake, errors.New("invalid pk")
	}

	if err := pk.Scan(&handshake.ProtocolVersion, &handshake.ServerAddress, &handshake.ServerPort, &handshake.NextState); err != nil {
		return handshake, err
	}

	return handshake, nil
}
