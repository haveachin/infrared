package status

import (
	"testing"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
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

		if pk.ID != IDServerBoundRequest {
			t.Error("invalid packet id")
		}
	}
}
