package login

import (
	"bytes"
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
)

func TestClientBoundDisconnect_Marshal(t *testing.T) {
	tt := []struct {
		packet          ClientBoundDisconnect
		marshaledPacket protocol.Packet
	}{
		{
			packet: ClientBoundDisconnect{
				Reason: protocol.Chat(""),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
		},
		{
			packet: ClientBoundDisconnect{
				Reason: protocol.Chat("Hello, World!"),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
		},
	}

	var pk protocol.Packet
	for _, tc := range tt {
		tc.packet.Marshal(&pk)

		if pk.ID != IDClientBoundDisconnect {
			t.Error("invalid packet id")
		}

		if !bytes.Equal(pk.Data, tc.marshaledPacket.Data) {
			t.Errorf("got: %v, want: %v", pk.Data, tc.marshaledPacket.Data)
		}
	}
}
