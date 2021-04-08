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

func TestVarInt_Encode(t *testing.T) {
	tt := []struct {
		given    VarInt
		expected []byte
	}{
		{
			given:    VarInt(0),
			expected: []byte{0x00},
		},
		{
			given:    VarInt(127),
			expected: []byte{0x7f},
		},
		{
			given:    VarInt(128),
			expected: []byte{0x80, 0x01},
		},
		{
			given:    VarInt(2097151),
			expected: []byte{0xff, 0xff, 0x7f},
		},
		{
			given:    VarInt(-1),
			expected: []byte{0xff, 0xff, 0xff, 0xff, 0x0f},
		},
	}

	for _, tc := range tt {
		if !bytes.Equal(tc.given.Encode(), tc.expected) {
			t.Fail()
		}
	}
}

func TestVarInt_Decode(t *testing.T) {
	tt := []struct {
		given    []byte
		expected VarInt
	}{
		{
			given:    []byte{0x00},
			expected: VarInt(0),
		},
		{
			given:    []byte{0x7f},
			expected: VarInt(127),
		},
		{
			given:    []byte{0x80, 0x01},
			expected: VarInt(128),
		},
		{
			given:    []byte{0xff, 0xff, 0x7f},
			expected: VarInt(2097151),
		},
		{
			given:    []byte{0xff, 0xff, 0xff, 0xff, 0x0f},
			expected: VarInt(-1),
		},
	}

	for _, tc := range tt {
		var actual VarInt
		if err := actual.Decode(bytes.NewReader(tc.given)); err != nil {
			t.Error(err)
		}

		if actual != tc.expected {
			t.Fail()
		}
	}
}
