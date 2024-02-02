package login

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const ServerBoundEncryptionResponseID int32 = 0x01

type ServerBoundEncryptionResponse struct {
	SharedSecret protocol.ByteArray
	VerifyToken  protocol.ByteArray
}

func (pk ServerBoundEncryptionResponse) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ServerBoundEncryptionResponseID,
		pk.SharedSecret,
		pk.VerifyToken,
	)
}

func (pk *ServerBoundEncryptionResponse) Unmarshal(packet protocol.Packet) error {
	if packet.ID != ServerBoundEncryptionResponseID {
		return protocol.ErrInvalidPacketID
	}

	return packet.Decode(
		&pk.SharedSecret,
		&pk.VerifyToken,
	)
}
