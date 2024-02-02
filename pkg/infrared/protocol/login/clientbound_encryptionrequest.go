package login

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const ClientBoundEncryptionRequestID int32 = 0x01

type ClientBoundEncryptionRequest struct {
	ServerID    protocol.String
	PublicKey   protocol.ByteArray
	VerifyToken protocol.ByteArray
}

func (pk ClientBoundEncryptionRequest) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ClientBoundEncryptionRequestID,
		pk.ServerID,
		pk.PublicKey,
		pk.VerifyToken,
	)
}

func (pk ClientBoundEncryptionRequest) Unmarshal(packet protocol.Packet) error {
	if packet.ID != ClientBoundEncryptionRequestID {
		return protocol.ErrInvalidPacketID
	}

	return packet.Decode(
		&pk.ServerID,
		&pk.PublicKey,
		&pk.VerifyToken,
	)
}
