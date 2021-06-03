package protocol_test

import (
	"bufio"
	"bytes"
	"net"
	"testing"

	"github.com/haveachin/infrared/protocol"
)

func TestPacket_Marshal(t *testing.T) {
	tt := []struct {
		packet   protocol.Packet
		expected []byte
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
			expected: []byte{0x03, 0x00, 0x00, 0xf2},
		},
		{
			packet: protocol.Packet{
				ID:   0x0f,
				Data: []byte{0x00, 0xf2, 0x03, 0x50},
			},
			expected: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
		},
	}

	for _, tc := range tt {
		actual, err := tc.packet.Marshal()
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(actual, tc.expected) {
			t.Errorf("got: %v; want: %v", actual, tc.expected)
		}
	}
}

func TestPacket_Scan(t *testing.T) {
	// Arrange
	packet := protocol.Packet{
		ID:   0x00,
		Data: []byte{0x00, 0xf2},
	}

	var booleanField protocol.Boolean
	var byteField protocol.Byte

	// Act
	err := packet.Scan(
		&booleanField,
		&byteField,
	)

	// Assert
	if err != nil {
		t.Error(err)
	}

	if booleanField != false {
		t.Error("got: true; want: false")
	}

	if !bytes.Equal(byteField.Encode(), []byte{0xf2}) {
		t.Errorf("got: %x; want: %x", byteField.Encode(), 0xf2)
	}
}

func TestScanFields(t *testing.T) {
	// Arrange
	packet := protocol.Packet{
		ID:   0x00,
		Data: []byte{0x00, 0xf2},
	}

	var booleanField protocol.Boolean
	var byteField protocol.Byte

	// Act
	err := protocol.ScanFields(
		bytes.NewReader(packet.Data),
		&booleanField,
		&byteField,
	)

	// Assert
	if err != nil {
		t.Error(err)
	}

	if booleanField != false {
		t.Error("got: true; want: false")
	}

	if !bytes.Equal(byteField.Encode(), []byte{0xf2}) {
		t.Errorf("got: %x; want: %x", byteField.Encode(), 0xf2)
	}
}

func TestMarshalPacket(t *testing.T) {
	// Arrange
	packetId := byte(0x00)
	booleanField := protocol.Boolean(false)
	byteField := protocol.Byte(0x0f)
	packetData := []byte{0x00, 0x0f}

	// Act
	packet := protocol.MarshalPacket(packetId, booleanField, byteField)

	// Assert
	if packet.ID != packetId {
		t.Errorf("packet id: got: %v; want: %v", packet.ID, packetId)
	}

	if !bytes.Equal(packet.Data, packetData) {
		t.Errorf("got: %v; want: %v", packet.Data, packetData)
	}
}

func TestReadPacketBytes(t *testing.T) {
	tt := []struct {
		data        []byte
		packetBytes []byte
	}{
		{
			data:        []byte{0x03, 0x00, 0x00, 0xf2, 0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
			packetBytes: []byte{0x00, 0x00, 0xf2},
		},
		{
			data:        []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50, 0x30, 0x01, 0xef, 0xaa},
			packetBytes: []byte{0x0f, 0x00, 0xf2, 0x03, 0x50},
		},
	}

	for _, tc := range tt {
		readBytes, err := protocol.ReadPacketBytes(bytes.NewReader(tc.data))
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(readBytes, tc.packetBytes) {
			t.Errorf("got: %v; want: %v", readBytes, tc.packetBytes)
		}
	}
}

func TestReadPacket(t *testing.T) {
	tt := []struct {
		data          []byte
		packet        protocol.Packet
		dataAfterRead []byte
	}{
		{
			data: []byte{0x03, 0x00, 0x00, 0xf2, 0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
			dataAfterRead: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
		},
		{
			data: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50, 0x30, 0x01, 0xef, 0xaa},
			packet: protocol.Packet{
				ID:   0x0f,
				Data: []byte{0x00, 0xf2, 0x03, 0x50},
			},
			dataAfterRead: []byte{0x30, 0x01, 0xef, 0xaa},
		},
	}

	for _, tc := range tt {
		buf := bytes.NewBuffer(tc.data)
		pk, err := protocol.ReadPacket(buf)
		if err != nil {
			t.Error(err)
		}

		if pk.ID != tc.packet.ID {
			t.Errorf("packet ID: got: %v; want: %v", pk.ID, tc.packet.ID)
		}

		if !bytes.Equal(pk.Data, tc.packet.Data) {
			t.Errorf("packet data: got: %v; want: %v", pk.Data, tc.packet.Data)
		}

		if !bytes.Equal(buf.Bytes(), tc.dataAfterRead) {
			t.Errorf("data after read: got: %v; want: %v", tc.data, tc.dataAfterRead)
		}
	}
}

func TestPeekPacket(t *testing.T) {
	tt := []struct {
		data   []byte
		packet protocol.Packet
	}{
		{
			data: []byte{0x03, 0x00, 0x00, 0xf2, 0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
		},
		{
			data: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50, 0x30, 0x01, 0xef, 0xaa},
			packet: protocol.Packet{
				ID:   0x0f,
				Data: []byte{0x00, 0xf2, 0x03, 0x50},
			},
		},
	}

	for _, tc := range tt {
		dataCopy := make([]byte, len(tc.data))
		copy(dataCopy, tc.data)

		pk, err := protocol.PeekPacket(bufio.NewReader(bytes.NewReader(dataCopy)))
		if err != nil {
			t.Error(err)
		}

		if pk.ID != tc.packet.ID {
			t.Errorf("packet ID: got: %v; want: %v", pk.ID, tc.packet.ID)
		}

		if !bytes.Equal(pk.Data, tc.packet.Data) {
			t.Errorf("packet data: got: %v; want: %v", pk.Data, tc.packet.Data)
		}

		if !bytes.Equal(dataCopy, tc.data) {
			t.Errorf("data after read: got: %v; want: %v", dataCopy, tc.data)
		}
	}
}

func benchmarkReadPacker(b *testing.B, amountBytes int) {
	data := []byte{}

	for i := 0; i < amountBytes; i++ {
		data = append(data, 0x01)
	}
	pk := protocol.Packet{ID: 0x05, Data: data}
	bytes, _ := pk.Marshal()
	c1, c2 := net.Pipe()
	r := bufio.NewReader(c1)

	go func() {
		for {
			c2.Write(bytes)
		}
	}()

	for n := 0; n < b.N; n++ {
		if _, err := protocol.ReadPacket(r); err != nil {
			b.Error(err)
		}
	}

}

func BenchmarkReadPacker_SingleByteVarInt(b *testing.B) {
	size := 0b0111111
	benchmarkReadPacker(b, size)
}

func BenchmarkReadPacker_DoubleByteVarInt(b *testing.B) {
	size := 0b1111111_0111111
	benchmarkReadPacker(b, size)
}

func BenchmarkReadPacker_TripleByteVarInt(b *testing.B) {
	size := 0b1111111_1111111_0111111
	benchmarkReadPacker(b, size)
}

func BenchmarkReadPacker_QuadrupleByteVarInt(b *testing.B) {
	size := 0b1111111_1111111_1111111_0111111
	benchmarkReadPacker(b, size)
}
