package status

import "github.com/haveachin/infrared/internal/pkg/java/protocol"

const (
	MaxSizeClientBoundResponse      = 1 + 32767*4
	IDClientBoundResponse      byte = 0x00
)

type ClientBoundResponse struct {
	JSONResponse protocol.String
}

func (pk ClientBoundResponse) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		IDClientBoundResponse,
		pk.JSONResponse,
	)
}

func UnmarshalClientBoundResponse(packet protocol.Packet) (ClientBoundResponse, error) {
	var pk ClientBoundResponse

	if packet.ID != IDClientBoundResponse {
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
	Version VersionJSON `json:"version"`
	Players PlayersJSON `json:"players"`
	// This has to be any to support the new chat style system
	Description any    `json:"description"`
	Favicon     string `json:"favicon,omitempty"`
	// Added since 1.19
	PreviewsChat bool `json:"previewsChat"`
	// Added since 1.19.1
	EnforcesSecureChat bool `json:"enforcesSecureChat"`
	// FMLModInfo should be set if the client is expecting a FML server
	// to response. This is necessary for the client to recognise the
	// server as a valid Forge server.
	FMLModInfo *FMLModInfoJSON `json:"modinfo,omitempty"`
	// FML2ForgeData should be set if the client is expecting a FML2 server
	// to response. This is necessary for the client to recognise the
	// server as a valid Forge server.
	FML2ForgeData *FML2ForgeDataJSON `json:"forgeData,omitempty"`
}

type VersionJSON struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type PlayersJSON struct {
	Max    int                `json:"max"`
	Online int                `json:"online"`
	Sample []PlayerSampleJSON `json:"sample,omitempty"`
}

type PlayerSampleJSON struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type DescriptionJSON struct {
	Text string `json:"text"`
}

// FMLModInfoJSON is a part of the FML Server List Ping
type FMLModInfoJSON struct {
	LoaderType string       `json:"type"`
	ModList    []FMLModJSON `json:"modList"`
}

type FMLModJSON struct {
	ID      string `json:"modid"`
	Version string `json:"version"`
}

// FML2ForgeDataJSON is a part of the FML2 Server List Ping
type FML2ForgeDataJSON struct {
	Channels          []FML2ChannelsJSON `json:"channels"`
	Mods              []FML2ModJSON      `json:"mods"`
	FMLNetworkVersion int                `json:"fmlNetworkVersion"`
	D                 string             `json:"d"`
}

type FML2ChannelsJSON struct {
	Res      string `json:"res"`
	Version  string `json:"version"`
	Required bool   `json:"required"`
}

type FML2ModJSON struct {
	ID     string `json:"modId"`
	Marker string `json:"modmarker"`
}
