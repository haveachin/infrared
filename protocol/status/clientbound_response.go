package status

import (
	"github.com/haveachin/infrared/protocol"
)

const ClientBoundResponsePacketID byte = 0x00

type ClientBoundResponse struct {
	JSONResponse protocol.String
}

func (pk ClientBoundResponse) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		ClientBoundResponsePacketID,
		pk.JSONResponse,
	)
}

func UnmarshalClientBoundResponse(packet protocol.Packet) (ClientBoundResponse, error) {
	var pk ClientBoundResponse

	if packet.ID != ClientBoundResponsePacketID {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.JSONResponse,
	); err != nil {
		return pk, err
	}

	return pk, nil
}

type ResponseJSON struct {
	Version     VersionJSON     `json:"version"`
	Players     PlayersJSON     `json:"players"`
	Description DescriptionJSON `json:"description"`
	Favicon     string          `json:"favicon"`
}

type VersionJSON struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type PlayersJSON struct {
	Max    int                `json:"max"`
	Online int                `json:"online"`
	Sample []PlayerSampleJSON `json:"sample"`
}

type PlayerSampleJSON struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type DescriptionJSON struct {
	Text string `json:"text"`
}
