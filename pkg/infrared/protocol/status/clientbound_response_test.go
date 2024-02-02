package status_test

import (
	"bytes"
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

func TestClientBoundResponse_Marshal(t *testing.T) {
	tt := []struct {
		packet          status.ClientBoundResponse
		marshaledPacket protocol.Packet
	}{
		{
			packet: status.ClientBoundResponse{
				JSONResponse: protocol.String(""),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
		},
		{
			packet: status.ClientBoundResponse{
				JSONResponse: protocol.String("Hello, World!"),
			},
			marshaledPacket: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
		},
	}

	var pk protocol.Packet
	for _, tc := range tt {
		_ = tc.packet.Marshal(&pk)

		if pk.ID != status.ClientBoundResponseID {
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
		unmarshalledPacket status.ClientBoundResponse
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			unmarshalledPacket: status.ClientBoundResponse{
				JSONResponse: "",
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			unmarshalledPacket: status.ClientBoundResponse{
				JSONResponse: protocol.String("Hello, World!"),
			},
		},
	}

	var actual status.ClientBoundResponse
	for _, tc := range tt {
		if err := actual.Unmarshal(tc.packet); err != nil {
			t.Error(err)
		}

		expected := tc.unmarshalledPacket

		if actual.JSONResponse != expected.JSONResponse {
			t.Errorf("got: %v, want: %v", actual, expected)
		}
	}
}
