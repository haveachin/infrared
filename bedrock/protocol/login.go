package protocol

import "log"

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
	return 0x01
}

func (pk *Login) Unmarshal(r *Reader) error {
	if err := r.BEInt32(&pk.ClientProtocol); err != nil {
		return err
	}
	if err := r.ByteSlice(&pk.ConnectionRequest); err != nil {
		return err
	}
	return nil
}

// Marshal ...
func (pk *Login) Marshal(buf *Writer) {
	log.Fatal("not implemented yet")
}
