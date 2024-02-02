package status_test

import (
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

func TestServerBoundRequest_Marshal(t *testing.T) {
	tt := []struct {
		packet          status.ServerBoundRequest
		marshaledPacket protocol.Packet
	}{
		{
			packet: status.ServerBoundRequest{},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{},
			},
		},
	}

	var pk protocol.Packet
	for _, tc := range tt {
		_ = tc.packet.Marshal(&pk)

		if pk.ID != status.ServerBoundRequestID {
			t.Error("invalid packet id")
		}
	}
}
