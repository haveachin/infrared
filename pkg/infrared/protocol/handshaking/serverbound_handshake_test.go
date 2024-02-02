package handshaking_test

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
)

func TestServerBoundHandshake_Marshal(t *testing.T) {
	tt := []struct {
		packet          handshaking.ServerBoundHandshake
		marshaledPacket protocol.Packet
	}{
		{
			packet: handshaking.ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      25565,
				NextState:       handshaking.StateStatusServerBoundHandshake,
			},
			marshaledPacket: protocol.Packet{
				ID: 0x00,
				Data: []byte{
					0xC2, 0x04, // ProtoVerion
					0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, // Server Address
					0x63, 0xDD, // Server Port
					0x01, // Next State
				},
			},
		},
		{
			packet: handshaking.ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      1337,
				NextState:       handshaking.StateStatusServerBoundHandshake,
			},
			marshaledPacket: protocol.Packet{
				ID: 0x00,
				Data: []byte{
					0xC2, 0x04, // ProtoVerion
					0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, // Server Address
					0x05, 0x39, // Server Port
					0x01, // Next State
				},
			},
		},
	}

	for _, tc := range tt {
		var pk protocol.Packet
		_ = tc.packet.Marshal(&pk)

		if pk.ID != handshaking.ServerBoundHandshakeID {
			t.Error("invalid packet id")
		}

		if !bytes.Equal(pk.Data, tc.marshaledPacket.Data) {
			t.Errorf("got: %v, want: %v", pk.Data, tc.marshaledPacket.Data)
		}
	}
}

func TestUnmarshalServerBoundHandshake(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		unmarshalledPacket handshaking.ServerBoundHandshake
	}{
		{
			packet: protocol.Packet{
				ID: 0x00,
				Data: []byte{
					0xC2, 0x04, // ProtoVerion
					0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, // Server Address
					0x63, 0xDD, // Server Port
					0x01, // Next State
				},
			},
			unmarshalledPacket: handshaking.ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      25565,
				NextState:       handshaking.StateStatusServerBoundHandshake,
			},
		},
		{
			packet: protocol.Packet{
				ID: 0x00,
				Data: []byte{
					0xC2, 0x04, // ProtoVerion
					0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, // Server Address
					0x05, 0x39, // Server Port
					0x01, // Next State
				},
			},
			unmarshalledPacket: handshaking.ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      1337,
				NextState:       handshaking.StateStatusServerBoundHandshake,
			},
		},
	}

	var actual handshaking.ServerBoundHandshake
	for _, tc := range tt {
		err := actual.Unmarshal(tc.packet)
		if err != nil {
			t.Error(err)
		}

		expected := tc.unmarshalledPacket

		if actual.ProtocolVersion != expected.ProtocolVersion ||
			actual.ServerAddress != expected.ServerAddress ||
			actual.ServerPort != expected.ServerPort ||
			actual.NextState != expected.NextState {
			t.Errorf("got: %v, want: %v", actual, tc.unmarshalledPacket)
		}
	}
}

func TestServerBoundHandshake_IsStatusRequest(t *testing.T) {
	tt := []struct {
		handshake handshaking.ServerBoundHandshake
		result    bool
	}{
		{
			handshake: handshaking.ServerBoundHandshake{
				NextState: handshaking.StateStatusServerBoundHandshake,
			},
			result: true,
		},
		{
			handshake: handshaking.ServerBoundHandshake{
				NextState: handshaking.StateLoginServerBoundHandshake,
			},
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.handshake.IsStatusRequest() != tc.result {
			t.Fail()
		}
	}
}

func TestServerBoundHandshake_IsLoginRequest(t *testing.T) {
	tt := []struct {
		handshake handshaking.ServerBoundHandshake
		result    bool
	}{
		{
			handshake: handshaking.ServerBoundHandshake{
				NextState: handshaking.StateStatusServerBoundHandshake,
			},
			result: false,
		},
		{
			handshake: handshaking.ServerBoundHandshake{
				NextState: handshaking.StateLoginServerBoundHandshake,
			},
			result: true,
		},
	}

	for _, tc := range tt {
		if tc.handshake.IsLoginRequest() != tc.result {
			t.Fail()
		}
	}
}

func TestServerBoundHandshake_IsForgeAddress(t *testing.T) {
	tt := []struct {
		addr   string
		result bool
	}{
		{
			addr:   handshaking.SeparatorForge,
			result: true,
		},
		{
			addr:   "example.com:1234" + handshaking.SeparatorForge,
			result: true,
		},
		{
			addr:   "example.com" + handshaking.SeparatorForge + "some data",
			result: true,
		},
		{
			addr:   "example.com" + handshaking.SeparatorForge + "some data" + handshaking.SeparatorRealIP + "more",
			result: true,
		},
		{
			addr:   "example.com",
			result: false,
		},
		{
			addr:   "",
			result: false,
		},
	}

	for _, tc := range tt {
		hs := handshaking.ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
		if hs.IsForgeAddress() != tc.result {
			t.Errorf("%s: got: %v; want: %v", tc.addr, !tc.result, tc.result)
		}
	}
}

func TestServerBoundHandshake_IsRealIPAddress(t *testing.T) {
	tt := []struct {
		addr   string
		result bool
	}{
		{
			addr:   handshaking.SeparatorRealIP,
			result: true,
		},
		{
			addr:   "example.com:25565" + handshaking.SeparatorRealIP,
			result: true,
		},
		{
			addr:   "example.com:1337" + handshaking.SeparatorRealIP + "some data",
			result: true,
		},
		{
			addr:   "example.com" + handshaking.SeparatorForge + "some data" + handshaking.SeparatorRealIP + "more",
			result: true,
		},
		{
			addr:   "example.com",
			result: false,
		},
		{
			addr:   ":1234",
			result: false,
		},
	}

	for _, tc := range tt {
		hs := handshaking.ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
		if hs.IsRealIPAddress() != tc.result {
			t.Errorf("%s: got: %v; want: %v", tc.addr, !tc.result, tc.result)
		}
	}
}

func TestServerBoundHandshake_ParseServerAddress(t *testing.T) {
	tt := []struct {
		addr         string
		expectedAddr string
	}{
		{
			addr:         "",
			expectedAddr: "",
		},
		{
			addr:         "example.com:25565",
			expectedAddr: "example.com:25565",
		},
		{
			addr:         handshaking.SeparatorForge,
			expectedAddr: "",
		},
		{
			addr:         handshaking.SeparatorRealIP,
			expectedAddr: "",
		},
		{
			addr:         "example.com" + handshaking.SeparatorForge,
			expectedAddr: "example.com",
		},
		{
			addr:         "example.com" + handshaking.SeparatorForge + "some data",
			expectedAddr: "example.com",
		},
		{
			addr:         "example.com:25565" + handshaking.SeparatorRealIP + "some data",
			expectedAddr: "example.com:25565",
		},
		{
			addr:         "example.com:1234" + handshaking.SeparatorForge + "some data" + handshaking.SeparatorRealIP + "more",
			expectedAddr: "example.com:1234",
		},
	}

	for _, tc := range tt {
		hs := handshaking.ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
		if hs.ParseServerAddress() != tc.expectedAddr {
			t.Errorf("got: %v; want: %v", hs.ParseServerAddress(), tc.expectedAddr)
		}
	}
}

func TestServerBoundHandshake_UpgradeToRealIP(t *testing.T) {
	tt := []struct {
		addr       string
		clientAddr net.TCPAddr
		timestamp  time.Time
	}{
		{
			addr: "example.com",
			clientAddr: net.TCPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: 12345,
			},
			timestamp: time.Now(),
		},
		{
			addr: "sub.example.com:25565",
			clientAddr: net.TCPAddr{
				IP:   net.IPv4(127, 0, 1, 1),
				Port: 25565,
			},
			timestamp: time.Now(),
		},
		{
			addr: "example.com:25565",
			clientAddr: net.TCPAddr{
				IP:   net.IPv4(127, 0, 2, 1),
				Port: 6543,
			},
			timestamp: time.Now(),
		},
		{
			addr: "example.com",
			clientAddr: net.TCPAddr{
				IP:   net.IPv4(127, 0, 3, 1),
				Port: 7467,
			},
			timestamp: time.Now(),
		},
	}

	for _, tc := range tt {
		hs := handshaking.ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
		hs.UpgradeToRealIP(&tc.clientAddr, tc.timestamp)

		if hs.ParseServerAddress() != tc.addr {
			t.Errorf("got: %v; want: %v", hs.ParseServerAddress(), tc.addr)
		}

		realIPSegments := strings.Split(string(hs.ServerAddress), handshaking.SeparatorRealIP)
		if len(realIPSegments) < 3 {
			t.Error("no real ip to test")
			return
		}

		if realIPSegments[1] != tc.clientAddr.String() {
			t.Errorf("got: %v; want: %v", realIPSegments[1], tc.addr)
		}

		unixTimestamp, err := strconv.ParseInt(realIPSegments[2], 10, 64)
		if err != nil {
			t.Error(err)
		}

		if unixTimestamp != tc.timestamp.Unix() {
			t.Errorf("timestamp is invalid: got: %d; want: %d", unixTimestamp, tc.timestamp.Unix())
		}
	}
}

func BenchmarkHandshakingServerBoundHandshake_Marshal(b *testing.B) {
	hsPk := handshaking.ServerBoundHandshake{
		ProtocolVersion: 578,
		ServerAddress:   "example.com",
		ServerPort:      25565,
		NextState:       1,
	}

	var pk protocol.Packet
	_ = hsPk.Marshal(&pk)

	for n := 0; n < b.N; n++ {
		if err := hsPk.Unmarshal(pk); err != nil {
			b.Error(err)
		}
	}
}
