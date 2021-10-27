package login

import (
	"bytes"
	"github.com/haveachin/infrared/protocol"
	"testing"
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

	for _, tc := range tt {
		pk := tc.packet.Marshal()

		if pk.ID != ClientBoundDisconnectPacketID {
			t.Error("invalid packet id")
		}

		if !bytes.Equal(pk.Data, tc.marshaledPacket.Data) {
			t.Errorf("got: %v, want: %v", pk.Data, tc.marshaledPacket.Data)
		}
	}
}
