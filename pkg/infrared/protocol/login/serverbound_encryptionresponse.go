package login

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const (
	IDServerBoundEncryptionResponse      int32 = 0x01
	MaxSizeServerBoundEncryptionResponse       = 1 + 5 + 128 + 5 + 128
)

type ServerBoundEncryptionResponse struct {
	SharedSecret protocol.ByteArray
	VerifyToken  protocol.ByteArray
}

func (pk ServerBoundEncryptionResponse) Marshal(packet *protocol.Packet) {
	packet.Encode(
		IDServerBoundEncryptionResponse,
		pk.SharedSecret,
		pk.VerifyToken,
	)
}

func (pk *ServerBoundEncryptionResponse) Unmarshal(packet protocol.Packet) error {
	if packet.ID != IDServerBoundEncryptionResponse {
		return protocol.ErrInvalidPacketID
	}

	return packet.Decode(
		&pk.SharedSecret,
		&pk.VerifyToken,
	)
}
