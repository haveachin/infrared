package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundEncryptionRequestPacketID byte = 0x01

type ClientBoundEncryptionRequest struct {
	ServerID    protocol.String
	PublicKey   protocol.ByteArray
	VerifyToken protocol.ByteArray
}

func (pk ClientBoundEncryptionRequest) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundEncryptionRequestPacketID,
		pk.ServerID,
		pk.PublicKey,
		pk.VerifyToken,
	)
}

func UnmarshalClientBoundEncryptionRequest(packet protocol.Packet) (ClientBoundEncryptionRequest, error) {
	var pk ClientBoundEncryptionRequest

	if packet.ID != ClientBoundEncryptionRequestPacketID {
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
