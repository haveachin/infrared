package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ServerBoundEncryptionResponsePacketID = 0x01

type ServerBoundEncryptionResponse struct {
	SharedSecret protocol.ByteArray
	VerifyToken  protocol.ByteArray
}

func (pk ServerBoundEncryptionResponse) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ServerBoundEncryptionResponsePacketID,
		pk.SharedSecret,
		pk.VerifyToken,
	)
}

func UnmarshalServerBoundEncryptionResponse(packet protocol.Packet) (ServerBoundEncryptionResponse, error) {
	var pk ServerBoundEncryptionResponse

	if packet.ID != ServerBoundEncryptionResponsePacketID {
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
