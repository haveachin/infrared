package packet

import (
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
)

// RequestNetworkSettings is sent by the client to request network settings, such as compression, from the server.
type RequestNetworkSettings struct {
	// ClientProtocol is the protocol version of the player. The player is disconnected if the protocol is
	// incompatible with the protocol of the server.
	ClientProtocol int32
}

// ID ...
func (pk *RequestNetworkSettings) ID() uint32 {
	return IDRequestNetworkSettings
}

// Marshal ...
func (pk *RequestNetworkSettings) Marshal(w *protocol.Writer) {
	w.BEInt32(&pk.ClientProtocol)
}

// Unmarshal ...
func (pk *RequestNetworkSettings) Unmarshal(r *protocol.Reader) error {
	return r.BEInt32(&pk.ClientProtocol)
}
