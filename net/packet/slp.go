package packet

const (
	SLPHandshakePacketID = 0x00
	SLPRequestPacketID   = 0x00
	SLPResponsePacketID  = 0x00
	SLPPingPacketID      = 0x01
	SLPPongPacketID      = 0x01

	SLPHandshakeStatusState = Byte(1)
	SLPHandshakeLoginState  = Byte(2)
)

type SLPHandshake struct {
	ProtocolVersion VarInt
	ServerAddress   String
	ServerPort      UnsignedShort
	NextState       Byte
}

func ParseSLPHandshake(pk Packet) (SLPHandshake, error) {
	var handshake SLPHandshake

	if pk.ID != SLPHandshakePacketID {
		return handshake, ErrInvalidPacketID
	}

	if err := pk.Scan(&handshake.ProtocolVersion, &handshake.ServerAddress, &handshake.ServerPort, &handshake.NextState); err != nil {
		return handshake, err
	}

	return handshake, nil
}

func (pk SLPHandshake) Marshal() Packet {
	return Marshal(SLPHandshakePacketID, pk.ProtocolVersion, pk.ServerAddress, pk.ServerPort, pk.NextState)
}

func (handshake SLPHandshake) RequestsStatus() bool {
	return handshake.NextState == SLPHandshakeStatusState
}

func (handshake SLPHandshake) RequestsLogin() bool {
	return handshake.NextState == SLPHandshakeLoginState
}

type SLPResponse struct {
	JSONResponse String
}

func (pk SLPResponse) Marshal() Packet {
	return Marshal(SLPResponsePacketID, pk.JSONResponse)
}
