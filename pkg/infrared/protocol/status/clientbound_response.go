package status

import "github.com/haveachin/infrared/pkg/infrared/protocol"

const (
	ClientBoundResponseID int32 = 0x00
)

type ClientBoundResponse struct {
	JSONResponse protocol.String
}

func (pk ClientBoundResponse) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ClientBoundResponseID,
		pk.JSONResponse,
	)
}

func (pk *ClientBoundResponse) Unmarshal(packet protocol.Packet) error {
	if packet.ID != ClientBoundResponseID {
		return protocol.ErrInvalidPacketID
	}

	return packet.Decode(
		&pk.JSONResponse,
	)
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

// FMLModInfoJSON is a part of the FML Server List Ping.
type FMLModInfoJSON struct {
	LoaderType string       `json:"type"`
	ModList    []FMLModJSON `json:"modList"`
}

type FMLModJSON struct {
	ID      string `json:"modid"`
	Version string `json:"version"`
}

// FML2ForgeDataJSON is a part of the FML2 Server List Ping.
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
