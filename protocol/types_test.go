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
			t.Errorf("reading bytes: %s", err)
		}

		if !bytes.Equal(bb, tc) {
			t.Errorf("got %v; want: %v", bb, tc)
		}
	}
}

var booleanTestTable = []struct {
	decoded Boolean
	encoded []byte
}{
	{
		decoded: Boolean(false),
		encoded: []byte{0x00},
	},
	{
		decoded: Boolean(true),
		encoded: []byte{0x01},
	},
}

func TestBoolean_Encode(t *testing.T) {
	for _, tc := range booleanTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestBoolean_Decode(t *testing.T) {
	for _, tc := range booleanTestTable {
		var actualDecoded Boolean
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var varIntTestTable = []struct {
	decoded VarInt
	encoded []byte
}{
	{
		decoded: VarInt(0),
		encoded: []byte{0x00},
	},
	{
		decoded: VarInt(1),
		encoded: []byte{0x01},
	},
	{
		decoded: VarInt(2),
		encoded: []byte{0x02},
	},
	{
		decoded: VarInt(127),
		encoded: []byte{0x7f},
	},
	{
		decoded: VarInt(128),
		encoded: []byte{0x80, 0x01},
	},
	{
		decoded: VarInt(255),
		encoded: []byte{0xff, 0x01},
	},
	{
		decoded: VarInt(2097151),
		encoded: []byte{0xff, 0xff, 0x7f},
	},
	{
		decoded: VarInt(2147483647),
		encoded: []byte{0xff, 0xff, 0xff, 0xff, 0x07},
	},
	{
		decoded: VarInt(-1),
		encoded: []byte{0xff, 0xff, 0xff, 0xff, 0x0f},
	},
	{
		decoded: VarInt(-2147483648),
		encoded: []byte{0x80, 0x80, 0x80, 0x80, 0x08},
	},
}

func TestVarInt_Encode(t *testing.T) {
	for _, tc := range varIntTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestVarInt_Decode(t *testing.T) {
	for _, tc := range varIntTestTable {
		var actualDecoded VarInt
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}
