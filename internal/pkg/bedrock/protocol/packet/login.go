package packet

import (
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
)

// Login is sent when the client initially tries to join the server. It is the first packet sent and contains
// information specific to the player.
type Login struct {
	// ClientProtocol is the protocol version of the player. The player is disconnected if the protocol is
	// incompatible with the protocol of the server.
	ClientProtocol int32
	// ConnectionRequest is a string containing information about the player and JWTs that may be used to
	// verify if the player is connected to XBOX Live. The connection request also contains the necessary
	// client public key to initiate encryption.
	ConnectionRequest []byte
}

func (pk *Login) ID() uint32 {
	return IDLogin
}

// Marshal ...
func (pk *Login) Marshal(w *protocol.Writer) {
	w.BEInt32(&pk.ClientProtocol)
	w.ByteSlice(&pk.ConnectionRequest)
}

func (pk *Login) Unmarshal(r *protocol.Reader) error {
	if err := r.BEInt32(&pk.ClientProtocol); err != nil {
		return err
	}
	if err := r.ByteSlice(&pk.ConnectionRequest); err != nil {
		return err
	}
	return nil
}
