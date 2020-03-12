package wrapper

import "github.com/Tnze/go-mc/net/packet"

const (
	SLPHandshakePacketID = 0x00
	SLPRequestPacketID   = 0x00
	SLPResponsePacketID  = 0x00
	SLPPingPacketID      = 0x01
	SLPPongPacketID      = 0x01

	SLPHandshakeStatusState = packet.Byte(1)
	SLPHandshakeLoginState  = packet.Byte(2)
)

type SLPHandshake struct {
	ProtocolVersion packet.VarInt
	ServerAddress   packet.String
	ServerPort      packet.UnsignedShort
	NextState       packet.Byte
}

func ParseSLPHandshake(pk packet.Packet) (SLPHandshake, error) {
	var handshake SLPHandshake

	if pk.ID != SLPHandshakePacketID {
		return handshake, ErrInvalidPacketID
	}

	if err := pk.Scan(&handshake.ProtocolVersion, &handshake.ServerAddress, &handshake.ServerPort, &handshake.NextState); err != nil {
		return handshake, err
	}

	return handshake, nil
}

func (pk SLPHandshake) Marshal() packet.Packet {
	return packet.Marshal(SLPHandshakePacketID, pk.ProtocolVersion, pk.ServerAddress, pk.ServerPort, pk.NextState)
}

func (handshake SLPHandshake) IsStatusRequest() bool {
	return handshake.NextState == SLPHandshakeStatusState
}

func (handshake SLPHandshake) IsLoginRequest() bool {
	return handshake.NextState == SLPHandshakeLoginState
}

type SLPResponse struct {
	JSONResponse packet.String
}

func (pk SLPResponse) Marshal() packet.Packet {
	return packet.Marshal(SLPResponsePacketID, pk.JSONResponse)
}
