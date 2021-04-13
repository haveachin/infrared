package handshaking

import (
	"bytes"
	"github.com/haveachin/infrared/protocol"
	"testing"
)

func TestServerBoundHandshake_Marshal(t *testing.T) {
	tt := []struct {
		packet          ServerBoundHandshake
		marshaledPacket protocol.Packet
	}{
		{
			packet: ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "spook.space",
				ServerPort:      25565,
				NextState:       ServerBoundHandshakeStatusState,
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0xC2, 0x04, 0x0B, 0x73, 0x70, 0x6F, 0x6F, 0x6B, 0x2E, 0x73, 0x70, 0x61, 0x63, 0x65, 0x63, 0xDD, 0x01},
			},
		},
		{
			packet: ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      1337,
				NextState:       ServerBoundHandshakeStatusState,
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0xC2, 0x04, 0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, 0x05, 0x39, 0x01},
			},
		},
	}

	for _, tc := range tt {
		pk := tc.packet.Marshal()

		if pk.ID != ServerBoundHandshakePacketID {
			t.Error("invalid packet id")
		}

		if !bytes.Equal(pk.Data, tc.marshaledPacket.Data) {
			t.Errorf("got: %v, want: %v", pk.Data, tc.marshaledPacket.Data)
		}
	}
}

func BenchmarkHandshakingServerBoundHandshake_Marshal(b *testing.B) {
	isHandshakePk := ServerBoundHandshake{
		ProtocolVersion: 578,
		ServerAddress:   "spook.space",
		ServerPort:      25565,
		NextState:       1,
	}

	pk := isHandshakePk.Marshal()

	for n := 0; n < b.N; n++ {
		if _, err := UnmarshalServerBoundHandshake(pk); err != nil {
			b.Error(err)
		}
	}
}
