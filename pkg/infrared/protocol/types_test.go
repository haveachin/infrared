package protocol_test

import (
	"bytes"
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
)

var varInts = []protocol.VarInt{0, 1, 2, 127, 128, 255, 2147483647, -1, -2147483648}

var packedVarInts = [][]byte{
	{0x00},
	{0x01},
	{0x02},
	{0x7f},
	{0x80, 0x01},
	{0xff, 0x01},
	{0xff, 0xff, 0xff, 0xff, 0x07},
	{0xff, 0xff, 0xff, 0xff, 0x0f},
	{0x80, 0x80, 0x80, 0x80, 0x08},
}

func TestVarInt_WriteTo(t *testing.T) {
	var buf bytes.Buffer
	for i, v := range varInts {
		buf.Reset()
		if n, err := v.WriteTo(&buf); err != nil {
			t.Fatalf("Write to bytes.Buffer should never fail: %v", err)
		} else if n != int64(buf.Len()) {
			t.Errorf("Number of byte returned by WriteTo should equal to buffer.Len()")
		}
		if p := buf.Bytes(); !bytes.Equal(p, packedVarInts[i]) {
			t.Errorf("pack int %d should be \"% x\", get \"% x\"", v, packedVarInts[i], p)
		}
	}
}

func TestVarInt_ReadFrom(t *testing.T) {
	for i, v := range packedVarInts {
		var vi protocol.VarInt
		if _, err := vi.ReadFrom(bytes.NewReader(v)); err != nil {
			t.Errorf("unpack \"% x\" error: %v", v, err)
		}
		if vi != varInts[i] {
			t.Errorf("unpack \"% x\" should be %d, get %d", v, varInts[i], vi)
		}
	}
}

func TestVarInt_ReadFrom_tooLongData(t *testing.T) {
	var vi protocol.VarInt
	data := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}
	if _, err := vi.ReadFrom(bytes.NewReader(data)); err != nil {
		t.Logf("unpack \"% x\" error: %v", data, err)
	} else {
		t.Errorf("unpack \"% x\" should be error, get %d", data, vi)
	}
}

func FuzzVarInt_Len(f *testing.F) {
	for _, v := range varInts {
		f.Add(int32(v))
	}
	var buf bytes.Buffer
	f.Fuzz(func(t *testing.T, v int32) {
		defer buf.Reset()
		if _, err := protocol.VarInt(v).WriteTo(&buf); err != nil {
			t.Fatal(err)
		}
		if a, b := buf.Len(), protocol.VarInt(v).Len(); a != b {
			t.Errorf("VarInt(%d) Length calculation error: calculated to be %d, actually %d", v, b, a)
		}
	})
}

func TestUnsignedShort_ReadFrom(t *testing.T) {
	tt := []struct {
		name     string
		expected protocol.UnsignedShort
		bb       []byte
	}{
		{
			name:     "zero",
			expected: 0,
			bb:       []byte{0x00, 0x00},
		},
		{
			name:     "255",
			expected: 255,
			bb:       []byte{0x00, 0xff},
		},
		{
			name:     "65280",
			expected: 65280,
			bb:       []byte{0xff, 0x00},
		},
		{
			name:     "3840",
			expected: 3840,
			bb:       []byte{0x0f, 0x00},
		},
		{
			name:     "65535",
			expected: 65535,
			bb:       []byte{0xff, 0xff},
		},
	}

	for _, tc := range tt {
		var actual protocol.UnsignedShort
		buf := bytes.NewBuffer(tc.bb)
		t.Run(tc.name, func(t *testing.T) {
			if n, err := actual.ReadFrom(buf); err != nil {
				t.Error(err)
			} else if n != 2 {
				t.Errorf("n != 2")
			}

			if actual != tc.expected {
				t.Errorf("want %d; got %d", tc.expected, actual)
			}
		})
	}
}

func TestUnsignedShort_WriteTo(t *testing.T) {
	tt := []struct {
		name     string
		us       protocol.UnsignedShort
		expected []byte
	}{
		{
			name:     "zero",
			us:       0,
			expected: []byte{0x00, 0x00},
		},
		{
			name:     "255",
			us:       255,
			expected: []byte{0x00, 0xff},
		},
		{
			name:     "65280",
			us:       65280,
			expected: []byte{0xff, 0x00},
		},
		{
			name:     "3840",
			us:       3840,
			expected: []byte{0x0f, 0x00},
		},
		{
			name:     "65535",
			us:       65535,
			expected: []byte{0xff, 0xff},
		},
	}

	for _, tc := range tt {
		var actual bytes.Buffer
		t.Run(tc.name, func(t *testing.T) {
			if n, err := tc.us.WriteTo(&actual); err != nil {
				t.Error(err)
			} else if n != 2 {
				t.Errorf("n != 2")
			}

			if !bytes.Equal(actual.Bytes(), tc.expected) {
				t.Errorf("want %d; got %d", tc.expected, actual.Bytes())
			}
		})
	}
}
