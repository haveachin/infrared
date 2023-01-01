package login

import "github.com/haveachin/infrared/internal/pkg/java/protocol"

const IDClientBoundEncryptionRequest byte = 0x01

type ClientBoundEncryptionRequest struct {
	ServerID    protocol.String
	PublicKey   protocol.ByteArray
	VerifyToken protocol.ByteArray
}

func (pk ClientBoundEncryptionRequest) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		IDClientBoundEncryptionRequest,
		pk.ServerID,
		pk.PublicKey,
		pk.VerifyToken,
	)
}

func UnmarshalClientBoundEncryptionRequest(packet protocol.Packet) (ClientBoundEncryptionRequest, error) {
	var pk ClientBoundEncryptionRequest

	if packet.ID != IDClientBoundEncryptionRequest {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.ServerID,
		&pk.PublicKey,
		&pk.VerifyToken,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
