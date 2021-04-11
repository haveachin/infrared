package protocol

import (
	"bytes"
	"github.com/gofrs/uuid"
	"io"
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

var stringTestTable = []struct {
	decoded String
	encoded []byte
}{
	{
		decoded: String(""),
		encoded: []byte{0x00},
	},
	{
		decoded: String("Hello, World!"),
		encoded: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
	},
	{
		decoded: String("Minecraft"),
		encoded: []byte{0x09, 0x4d, 0x69, 0x6e, 0x65, 0x63, 0x72, 0x61, 0x66, 0x74},
	},
	{
		decoded: String("♥"),
		encoded: []byte{0x03, 0xe2, 0x99, 0xa5},
	},
}

func TestString_Encode(t *testing.T) {
	for _, tc := range stringTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestString_Decode(t *testing.T) {
	for _, tc := range stringTestTable {
		var actualDecoded String
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var byteTestTable = []struct {
	decoded Byte
	encoded []byte
}{
	{
		decoded: Byte(0x00),
		encoded: []byte{0x00},
	},
	{
		decoded: Byte(0x0f),
		encoded: []byte{0x0f},
	},
}

func TestByte_Encode(t *testing.T) {
	for _, tc := range byteTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestByte_Decode(t *testing.T) {
	for _, tc := range byteTestTable {
		var actualDecoded Byte
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var unsignedShortTestTable = []struct {
	decoded UnsignedShort
	encoded []byte
}{
	{
		decoded: UnsignedShort(0),
		encoded: []byte{0x00, 0x00},
	},
	{
		decoded: UnsignedShort(15),
		encoded: []byte{0x00, 0x0f},
	},
	{
		decoded: UnsignedShort(16),
		encoded: []byte{0x00, 0x10},
	},
	{
		decoded: UnsignedShort(255),
		encoded: []byte{0x00, 0xff},
	},
	{
		decoded: UnsignedShort(256),
		encoded: []byte{0x01, 0x00},
	},
	{
		decoded: UnsignedShort(65535),
		encoded: []byte{0xff, 0xff},
	},
}

func TestUnsignedShort_Encode(t *testing.T) {
	for _, tc := range unsignedShortTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestUnsignedShort_Decode(t *testing.T) {
	for _, tc := range unsignedShortTestTable {
		var actualDecoded UnsignedShort
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var longTestTable = []struct {
	decoded Long
	encoded []byte
}{
	{
		decoded: Long(-9223372036854775808),
		encoded: []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		decoded: Long(0),
		encoded: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		decoded: Long(15),
		encoded: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0f},
	},
	{
		decoded: Long(16),
		encoded: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10},
	},
	{
		decoded: Long(9223372036854775807),
		encoded: []byte{0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	},
}

func TestLong_Encode(t *testing.T) {
	for _, tc := range longTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestLong_Decode(t *testing.T) {
	for _, tc := range longTestTable {
		var actualDecoded Long
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			t.Errorf("decoding: %s", err)
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var byteArrayTestTable = []struct {
	decoded ByteArray
	encoded []byte
}{
	{
		decoded: ByteArray([]byte{}),
		encoded: []byte{0x00},
	},
	{
		decoded: ByteArray([]byte{0x00}),
		encoded: []byte{0x01, 0x00},
	},
}

func TestByteArray_Encode(t *testing.T) {
	for _, tc := range byteArrayTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestByteArray_Decode(t *testing.T) {
	for _, tc := range byteArrayTestTable {
		actualDecoded := ByteArray([]byte{})
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			if err != io.EOF {
				t.Errorf("decoding: %s", err)
			}
		}

		if !bytes.Equal(actualDecoded, tc.decoded) {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var optionalByteArrayTestTable = []struct {
	decoded OptionalByteArray
	encoded []byte
}{
	{
		decoded: OptionalByteArray([]byte{}),
		encoded: []byte{},
	},
	{
		decoded: OptionalByteArray([]byte{0x00}),
		encoded: []byte{0x00},
	},
}

func TestOptionalByteArray_Encode(t *testing.T) {
	for _, tc := range optionalByteArrayTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestOptionalByteArray_Decode(t *testing.T) {
	for _, tc := range optionalByteArrayTestTable {
		actualDecoded := OptionalByteArray([]byte{})
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			if err != io.EOF {
				t.Errorf("decoding: %s", err)
			}
		}

		if !bytes.Equal(actualDecoded, tc.decoded) {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}

var uuidTestTable = []struct {
	decoded UUID
	encoded []byte
}{
	{
		decoded: UUID(uuid.UUID{}),
		encoded: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		decoded: UUID(uuid.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}),
		encoded: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	},
}

func TestUUID_Encode(t *testing.T) {
	for _, tc := range uuidTestTable {
		if !bytes.Equal(tc.decoded.Encode(), tc.encoded) {
			t.Errorf("encoding: got: %v; want: %v", tc.decoded.Encode(), tc.encoded)
		}
	}
}

func TestUUID_Decode(t *testing.T) {
	for _, tc := range uuidTestTable {
		var actualDecoded UUID
		if err := actualDecoded.Decode(bytes.NewReader(tc.encoded)); err != nil {
			if err != io.EOF {
				t.Errorf("decoding: %s", err)
			}
		}

		if actualDecoded != tc.decoded {
			t.Errorf("decoding: got %v; want: %v", actualDecoded, tc.decoded)
		}
	}
}
