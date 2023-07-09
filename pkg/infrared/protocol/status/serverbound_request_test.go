package status

import (
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
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

	var pk protocol.Packet
	for _, tc := range tt {
		tc.packet.Marshal(&pk)

		if pk.ID != IDServerBoundRequest {
			t.Error("invalid packet id")
		}
	}
}
