package login

import (
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
)

func TestUnmarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		version            protocol.Version
		unmarshalledPacket ServerBoundLoginStart
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			version: protocol.Version_1_18_2,
			unmarshalledPacket: ServerBoundLoginStart{
				Name: protocol.String(""),
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			version: protocol.Version_1_18_2,
			unmarshalledPacket: ServerBoundLoginStart{
				Name: protocol.String("Hello, World!"),
			},
		},
	}

	var loginStart ServerBoundLoginStart
	for _, tc := range tt {
		if err := loginStart.Unmarshal(tc.packet, tc.version); err != nil {
			t.Error(err)
		}

		if loginStart.Name != tc.unmarshalledPacket.Name {
			t.Errorf("got: %v, want: %v", loginStart.Name, tc.unmarshalledPacket.Name)
		}
	}
}
