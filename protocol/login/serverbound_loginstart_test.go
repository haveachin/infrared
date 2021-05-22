package login_test

import (
	"bytes"
	"testing"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/login"
)

func TestMarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		mcName string
	}{
		{
			mcName: "test",
		},
		{
			mcName: "infrared",
		},
		{
			mcName: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.mcName, func(t *testing.T) {
			expectedPk := protocol.Packet{
				ID:   login.ServerBoundLoginStartPacketID,
				Data: []byte(tc.mcName),
			}

			loginStart := login.ServerLoginStart{}
			pk := loginStart.Marshal()

			if expectedPk.ID != pk.ID || bytes.Equal(expectedPk.Data, pk.Data) {
				t.Logf("expected:\t%v", expectedPk)
				t.Logf("got:\t\t%v", pk)
				t.Error("Difference be expected and received packet")
			}
		})
	}

}

func TestUnmarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		unmarshalledPacket login.ServerLoginStart
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			unmarshalledPacket: login.ServerLoginStart{
				Name: protocol.String(""),
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			unmarshalledPacket: login.ServerLoginStart{
				Name: protocol.String("Hello, World!"),
			},
		},
	}

	for _, tc := range tt {
		loginStart, err := login.UnmarshalServerBoundLoginStart(tc.packet)
		if err != nil {
			t.Error(err)
		}

		if loginStart.Name != tc.unmarshalledPacket.Name {
			t.Errorf("got: %v, want: %v", loginStart.Name, tc.unmarshalledPacket.Name)
		}
	}
}
