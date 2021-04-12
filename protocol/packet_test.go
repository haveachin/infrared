package protocol

import (
	"bytes"
	"testing"
)

func TestPacket_Marshal(t *testing.T) {
	tt := []struct {
		packet    Packet
		expected  []byte
		threshold int
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
