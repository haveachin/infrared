package protocol

import (
	"bytes"
	"testing"
)

func TestReadNBytes(t *testing.T) {
	tt := [][]byte{
		{0x00, 0x01, 0x02, 0x03},
		{0x03, 0x01, 0x02, 0x02},
	}

	for _, tc := range tt {
		bb, err := ReadNBytes(bytes.NewBuffer(tc), len(tc))
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(bb, tc) {
			t.Fail()
		}
	}
}

func TestBoolean_Encode(t *testing.T) {
	tt := []struct {
		given    Boolean
		expected []byte
	}{
		{
			given:    Boolean(false),
			expected: []byte{0x00},
		},
		{
			given:    Boolean(true),
			expected: []byte{0x01},
		},
	}

	for _, tc := range tt {
		if !bytes.Equal(tc.given.Encode(), tc.expected) {
			t.Fail()
		}
	}
}

func TestBoolean_Decode(t *testing.T) {
	tt := []struct {
		given    []byte
		expected Boolean
	}{
		{
			given:    []byte{0x00},
			expected: Boolean(false),
		},
		{
			given:    []byte{0x01},
			expected: Boolean(true),
		},
	}

	for _, tc := range tt {
		var actual Boolean
		if err := actual.Decode(bytes.NewReader(tc.given)); err != nil {
			t.Error(err)
		}

		if actual != tc.expected {
			t.Fail()
		}
	}
}
