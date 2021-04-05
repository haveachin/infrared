package login

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundLoginPluginRequestPacketID byte = 0x04

type ClientBoundLoginPluginRequest struct {
	MessageID protocol.VarInt
	Channel   protocol.Identifier
	Data      protocol.OptionalByteArray
}

func (pk ClientBoundLoginPluginRequest) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundLoginPluginRequestPacketID,
		pk.MessageID,
		pk.Channel,
		pk.Data,
	)
}

func UnmarshalClientBoundLoginPluginRequest(packet protocol.Packet) (ClientBoundLoginPluginRequest, error) {
	var pk ClientBoundLoginPluginRequest

	if packet.ID != ClientBoundLoginPluginRequestPacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.MessageID,
		&pk.Channel,
		&pk.Data,
	); err != nil {
		return pk, err
	}

	return pk, nil
}
