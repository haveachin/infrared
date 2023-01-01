package login

import "github.com/haveachin/infrared/internal/pkg/java/protocol"

const (
	IDServerBoundEncryptionResponse = 0x01
	MaxSizeServerBoundEncryptionResponse
)

type ServerBoundEncryptionResponse struct {
	SharedSecret protocol.ByteArray
	VerifyToken  protocol.ByteArray
}

func (pk ServerBoundEncryptionResponse) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		IDServerBoundEncryptionResponse,
		pk.SharedSecret,
		pk.VerifyToken,
	)
}

func UnmarshalServerBoundEncryptionResponse(packet protocol.Packet) (ServerBoundEncryptionResponse, error) {
	var pk ServerBoundEncryptionResponse

	if packet.ID != IDServerBoundEncryptionResponse {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.SharedSecret,
		&pk.VerifyToken,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
