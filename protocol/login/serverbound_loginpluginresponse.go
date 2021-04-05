package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ServerBoundLoginPluginResponsePacketID byte = 0x04

type ServerBoundLoginPluginResponse struct {
	MessageID  protocol.VarInt
	Successful protocol.Boolean
	Data       protocol.OptionalByteArray
}

func (pk ServerBoundLoginPluginResponse) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ServerBoundLoginPluginResponsePacketID,
		pk.MessageID,
		pk.Successful,
		pk.Data,
	)
}

func UnmarshalServerBoundLoginPluginResponse(packet protocol.Packet) (ServerBoundLoginPluginResponse, error) {
	var pk ServerBoundLoginPluginResponse

	if packet.ID != ServerBoundLoginPluginResponsePacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.MessageID,
		&pk.Successful,
		&pk.Data,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
