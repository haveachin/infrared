package handshaking

import (
	"bytes"
	"github.com/haveachin/infrared/protocol"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestUnmarshalServerBoundHandshake(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		unmarshalledPacket ServerBoundHandshake
	}{
		{
			packet: protocol.Packet{
				ID: 0x00,
				//           ProtoVer. | Server Address                                                        |Serv. Port | Nxt State
				Data: []byte{0xC2, 0x04, 0x0B, 0x73, 0x70, 0x6F, 0x6F, 0x6B, 0x2E, 0x73, 0x70, 0x61, 0x63, 0x65, 0x63, 0xDD, 0x01},
			},
			unmarshalledPacket: ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "spook.space",
				ServerPort:      25565,
				NextState:       ServerBoundHandshakeStatusState,
			},
		},
		{
			packet: protocol.Packet{
				ID: 0x00,
				//           ProtoVer. | Server Address                                                        |Serv. Port | Nxt State
				Data: []byte{0xC2, 0x04, 0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, 0x05, 0x39, 0x01},
			},
			unmarshalledPacket: ServerBoundHandshake{
				ProtocolVersion: 578,
				ServerAddress:   "example.com",
				ServerPort:      1337,
				NextState:       ServerBoundHandshakeStatusState,
			},
		},
	}

	for _, tc := range tt {
		actual, err := UnmarshalServerBoundHandshake(tc.packet)
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
		handshake ServerBoundHandshake
		result    bool
	}{
		{
			handshake: ServerBoundHandshake{
				NextState: ServerBoundHandshakeStatusState,
			},
			result: true,
		},
		{
			handshake: ServerBoundHandshake{
				NextState: ServerBoundHandshakeLoginState,
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
		handshake ServerBoundHandshake
		result    bool
	}{
		{
			handshake: ServerBoundHandshake{
				NextState: ServerBoundHandshakeStatusState,
			},
			result: false,
		},
		{
			handshake: ServerBoundHandshake{
				NextState: ServerBoundHandshakeLoginState,
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
			addr:   ForgeSeparator,
			result: true,
		},
		{
			addr:   "example.com:1234" + ForgeSeparator,
			result: true,
		},
		{
			addr:   "example.com" + ForgeSeparator + "some data",
			result: true,
		},
		{
			addr:   "example.com" + ForgeSeparator + "some data" + RealIPSeparator + "more",
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
		hs := ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
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
			addr:   RealIPSeparator,
			result: true,
		},
		{
			addr:   "example.com:25565" + RealIPSeparator,
			result: true,
		},
		{
			addr:   "example.com:1337" + RealIPSeparator + "some data",
			result: true,
		},
		{
			addr:   "example.com" + ForgeSeparator + "some data" + RealIPSeparator + "more",
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
		hs := ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
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
			addr:         ForgeSeparator,
			expectedAddr: "",
		},
		{
			addr:         RealIPSeparator,
			expectedAddr: "",
		},
		{
			addr:         "example.com" + ForgeSeparator,
			expectedAddr: "example.com",
		},
		{
			addr:         "example.com" + ForgeSeparator + "some data",
			expectedAddr: "example.com",
		},
		{
			addr:         "example.com:25565" + RealIPSeparator + "some data",
			expectedAddr: "example.com:25565",
		},
		{
			addr:         "example.com:1234" + ForgeSeparator + "some data" + RealIPSeparator + "more",
			expectedAddr: "example.com:1234",
		},
	}

	for _, tc := range tt {
		hs := ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
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
		hs := ServerBoundHandshake{ServerAddress: protocol.String(tc.addr)}
		hs.UpgradeToRealIP(&tc.clientAddr, tc.timestamp)

		if hs.ParseServerAddress() != tc.addr {
			t.Errorf("got: %v; want: %v", hs.ParseServerAddress(), tc.addr)
		}

		realIpSegments := strings.Split(string(hs.ServerAddress), RealIPSeparator)

		if realIpSegments[1] != tc.clientAddr.String() {
			t.Errorf("got: %v; want: %v", realIpSegments[1], tc.addr)
		}

		unixTimestamp, err := strconv.ParseInt(realIpSegments[2], 10, 64)
		if err != nil {
			t.Error(err)
		}

		if unixTimestamp != tc.timestamp.Unix() {
			t.Errorf("timestamp is invalid: got: %d; want: %d", unixTimestamp, tc.timestamp.Unix())
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
