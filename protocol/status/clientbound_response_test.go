package status

import (
	"bytes"
	"github.com/haveachin/infrared/protocol"
	"testing"
)

func TestClientBoundResponse_Marshal(t *testing.T) {
	tt := []struct {
		packet          ClientBoundResponse
		marshaledPacket protocol.Packet
	}{
		{
			packet: ClientBoundResponse{
				JSONResponse: protocol.String(""),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
		},
		{
			packet: ClientBoundResponse{
				JSONResponse: protocol.String("Hello, World!"),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
		},
	}

	for _, tc := range tt {
		pk := tc.packet.Marshal()

		if pk.ID != ClientBoundResponsePacketID {
			t.Error("invalid packet id")
		}

		if !bytes.Equal(pk.Data, tc.marshaledPacket.Data) {
			t.Errorf("got: %v, want: %v", pk.Data, tc.marshaledPacket.Data)
		}
	}
}

func TestUnmarshalClientBoundResponse(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		unmarshalledPacket ClientBoundResponse
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			unmarshalledPacket: ClientBoundResponse{
				JSONResponse: "",
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			unmarshalledPacket: ClientBoundResponse{
				JSONResponse: protocol.String("Hello, World!"),
			},
		},
	}

	for _, tc := range tt {
		actual, err := UnmarshalClientBoundResponse(tc.packet)
		if err != nil {
			t.Error(err)
		}

		expected := tc.unmarshalledPacket

		if actual.JSONResponse != expected.JSONResponse {
			t.Errorf("got: %v, want: %v", actual, expected)
		}
	}

}
