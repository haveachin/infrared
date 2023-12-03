package protocol

import (
	"bytes"
	"io"
	"testing"
)

type result struct {
	match bool
	n     int64
	err   error
}

type booleanTC struct {
	name string
	v    Boolean
	b    []byte
	r    result
}

var booleanTT = []booleanTC{
	{
		name: "False is 0x00",
		v:    Boolean(false),
		b:    []byte{0x00},
		r: result{
			match: true,
			n:     1,
			err:   nil,
		},
	},
	{
		name: "False is not 0x01",
		v:    Boolean(false),
		b:    []byte{0x01},
		r: result{
			match: false,
			n:     1,
			err:   nil,
		},
	},
	{
		name: "True is not 0x00",
		v:    Boolean(true),
		b:    []byte{0x00},
		r: result{
			match: false,
			n:     1,
			err:   nil,
		},
	},
	{
		name: "True is 0x01",
		v:    Boolean(true),
		b:    []byte{0x01},
		r: result{
			match: true,
			n:     1,
			err:   nil,
		},
	},
}

func TestBoolean_WriteTo(t *testing.T) {
	buf := &bytes.Buffer{}
	for _, tc := range booleanTT {
		t.Run(tc.name, func(t *testing.T) {
			n, err := tc.v.WriteTo(buf)
			if err != tc.r.err {
				t.Errorf("expected: %s; got: %s", tc.r.err, err)
			}

			if n != tc.r.n {
				t.Errorf("expected: %d; got: %d", tc.r.n, n)
			}

			if bytes.Equal(buf.Bytes(), tc.b) != tc.r.match {
				t.Errorf("bytes equality: expected: %v; got: %v", tc.r.match, !tc.r.match)
			}
		})
		buf.Reset()
	}
}

func TestBoolean_ReadFrom(t *testing.T) {
	booleanTT = append(booleanTT, []booleanTC{
		{
			name: "False no bytes",
			v:    Boolean(false),
			b:    []byte{},
			r: result{
				match: true,
				n:     0,
				err:   io.EOF,
			},
		},
		{
			name: "True no bytes",
			v:    Boolean(true),
			b:    []byte{},
			r: result{
				match: false,
				n:     0,
				err:   io.EOF,
			},
		},
	}...)

	for _, tc := range booleanTT {
		var v Boolean
		t.Run(tc.name, func(t *testing.T) {
			n, err := v.ReadFrom(bytes.NewBuffer(tc.b))
			if err != tc.r.err {
				t.Errorf("expected: %s; got: %s", tc.r.err, err)
			}

			if n != tc.r.n {
				t.Errorf("expected: %d; got: %d", tc.r.n, n)
			}

			if v == tc.v != tc.r.match {
				t.Errorf("value equality: expected: %v; got: %v", tc.r.match, !tc.r.match)
			}
		})
	}
}

func BenchmarkBoolean_ReadFrom(b *testing.B) {
	var v Boolean
	bb := []byte{0x00}
	buf := bytes.NewBuffer(make([]byte, 0, 1))

	for i := 0; i < b.N; i++ {
		buf.Write(bb)
		v.ReadFrom(buf)
	}
}

type stringTC struct {
	name string
	v    String
	b    []byte
	r    result
}

var stringTT = []stringTC{
	{
		name: "Empty String is no bytes",
		v:    "",
		b:    []byte{0x00},
		r: result{
			match: true,
			n:     1,
			err:   nil,
		},
	},
	{
		name: "Hello, World!",
		v:    "Hello, World!",
		b:    []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
		r: result{
			match: true,
			n:     14,
			err:   nil,
		},
	},
	{
		name: "Minecraft",
		v:    "Minecraft",
		b:    []byte{0x09, 0x4d, 0x69, 0x6e, 0x65, 0x63, 0x72, 0x61, 0x66, 0x74},
		r: result{
			match: true,
			n:     10,
			err:   nil,
		},
	},
	{
		name: "♥",
		v:    "♥",
		b:    []byte{0x03, 0xe2, 0x99, 0xa5},
		r: result{
			match: true,
			n:     4,
			err:   nil,
		},
	},
}

func TestString_WriteTo(t *testing.T) {
	stringTT = append(stringTT, []stringTC{
		{},
	}...)

	buf := &bytes.Buffer{}
	for _, tc := range stringTT {
		t.Run(tc.name, func(t *testing.T) {
			n, err := tc.v.WriteTo(buf)
			if err != tc.r.err {
				t.Errorf("expected: %s; got: %s", tc.r.err, err)
			}

			if n != tc.r.n {
				t.Errorf("expected: %d; got: %d", tc.r.n, n)
			}

			if bytes.Equal(buf.Bytes(), tc.b) != tc.r.match {
				t.Errorf("bytes equality: expected: %v; got: %v", tc.r.match, !tc.r.match)
			}
		})
		buf.Reset()
	}
}

func TestString_ReadFrom(t *testing.T) {
	for _, tc := range stringTT {
		var v String
		t.Run(tc.name, func(t *testing.T) {
			n, err := v.ReadFrom(bytes.NewBuffer(tc.b))
			if err != tc.r.err {
				t.Errorf("expected: %s; got: %s", tc.r.err, err)
			}

			if n != tc.r.n {
				t.Errorf("expected: %d; got: %d", tc.r.n, n)
			}

			if v == tc.v != tc.r.match {
				t.Errorf("value equality: expected: %v; got: %v", tc.r.match, !tc.r.match)
			}
		})
	}
}

func BenchmarkString_ReadFrom(b *testing.B) {
	var v String
	bb := []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21}
	buf := bytes.NewBuffer(make([]byte, 0, len(bb)))

	for i := 0; i < b.N; i++ {
		buf.Write(bb)
		v.ReadFrom(buf)
	}
}

func BenchmarkString_WriteTo(b *testing.B) {
	v := String("Hello, World!")
	bb := []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21}
	buf := bytes.NewBuffer(make([]byte, 0, len(bb)))

	for i := 0; i < b.N; i++ {
		v.WriteTo(buf)
		buf.Reset()
	}
}
