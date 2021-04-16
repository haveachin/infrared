package status

import (
	"github.com/haveachin/infrared/protocol"
	"testing"
)

func TestServerBoundRequest_Marshal(t *testing.T) {
	tt := []struct {
		packet          ServerBoundRequest
		marshaledPacket protocol.Packet
	}{
		{
			packet: ServerBoundRequest{},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{},
			},
		},
	}

	for _, tc := range tt {
		pk := tc.packet.Marshal()

		if pk.ID != ServerBoundRequestPacketID {
			t.Error("invalid packet id")
		}
	}
}
