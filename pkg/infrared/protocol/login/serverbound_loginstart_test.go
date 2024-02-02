package login_test

import (
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/login"
)

func TestUnmarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		version            protocol.Version
		unmarshalledPacket login.ServerBoundLoginStart
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			version: protocol.Version1_18_2,
			unmarshalledPacket: login.ServerBoundLoginStart{
				Name: protocol.String(""),
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			version: protocol.Version1_18_2,
			unmarshalledPacket: login.ServerBoundLoginStart{
				Name: protocol.String("Hello, World!"),
			},
		},
	}

	var loginStart login.ServerBoundLoginStart
	for _, tc := range tt {
		if err := loginStart.Unmarshal(tc.packet, tc.version); err != nil {
			t.Error(err)
		}

		if loginStart.Name != tc.unmarshalledPacket.Name {
			t.Errorf("got: %v, want: %v", loginStart.Name, tc.unmarshalledPacket.Name)
		}
	}
}
