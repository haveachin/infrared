package protocol

import (
	"bufio"
	"bytes"
	"testing"
)

func TestPacket_Marshal(t *testing.T) {
	tt := []struct {
		packet   Packet
		expected []byte
	}{
		{
			packet: Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
			expected: []byte{0x03, 0x00, 0x00, 0xf2},
		},
		{
			packet: Packet{
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
	packet := Packet{
		ID:   0x00,
		Data: []byte{0x00, 0xf2},
	}

	var booleanField Boolean
	var byteField Byte

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
	packet := Packet{
		ID:   0x00,
		Data: []byte{0x00, 0xf2},
	}

	var booleanField Boolean
	var byteField Byte

	// Act
	err := ScanFields(
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
	booleanField := Boolean(false)
	byteField := Byte(0x0f)
	packetData := []byte{0x00, 0x0f}

	// Act
	packet := MarshalPacket(packetId, booleanField, byteField)

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
		readBytes, err := ReadPacketBytes(bytes.NewReader(tc.data))
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
		packet        Packet
		dataAfterRead []byte
	}{
		{
			data: []byte{0x03, 0x00, 0x00, 0xf2, 0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
			packet: Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
			dataAfterRead: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
		},
		{
			data: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50, 0x30, 0x01, 0xef, 0xaa},
			packet: Packet{
				ID:   0x0f,
				Data: []byte{0x00, 0xf2, 0x03, 0x50},
			},
			dataAfterRead: []byte{0x30, 0x01, 0xef, 0xaa},
		},
	}

	for _, tc := range tt {
		buf := bytes.NewBuffer(tc.data)
		pk, err := ReadPacket(buf)
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
		packet Packet
	}{
		{
			data: []byte{0x03, 0x00, 0x00, 0xf2, 0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50},
			packet: Packet{
				ID:   0x00,
				Data: []byte{0x00, 0xf2},
			},
		},
		{
			data: []byte{0x05, 0x0f, 0x00, 0xf2, 0x03, 0x50, 0x30, 0x01, 0xef, 0xaa},
			packet: Packet{
				ID:   0x0f,
				Data: []byte{0x00, 0xf2, 0x03, 0x50},
			},
		},
	}

	for _, tc := range tt {
		dataCopy := make([]byte, len(tc.data))
		copy(dataCopy, tc.data)

		pk, err := PeekPacket(bufio.NewReader(bytes.NewReader(dataCopy)))
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
