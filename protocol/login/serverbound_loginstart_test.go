package login

import (
	"github.com/haveachin/infrared/protocol"
	"testing"
)

func TestUnmarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		unmarshalledPacket ServerLoginStart
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			unmarshalledPacket: ServerLoginStart{
				Name: protocol.String(""),
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			unmarshalledPacket: ServerLoginStart{
				Name: protocol.String("Hello, World!"),
			},
		},
	}

	for _, tc := range tt {
		loginStart, err := UnmarshalServerBoundLoginStart(tc.packet)
		if err != nil {
			t.Error(err)
		}

		if loginStart.Name != tc.unmarshalledPacket.Name {
			t.Errorf("got: %v, want: %v", loginStart.Name, tc.unmarshalledPacket.Name)
		}
	}
}
