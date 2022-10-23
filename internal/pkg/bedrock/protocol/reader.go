package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"unsafe"
)

type DecodeReader interface {
	io.Reader
	io.ByteReader
}

type Reader struct {
	DecodeReader
}

func NewReader(r DecodeReader) *Reader {
	return &Reader{DecodeReader: r}
}

// Uint8 reads a uint8 from the underlying buffer.
func (r *Reader) Uint8(x *uint8) error {
	var err error
	*x, err = r.ReadByte()
	if err != nil {
		return err
	}
	return nil
}

// Uint16 reads a little endian uint16 from the underlying buffer.
func (r *Reader) Uint16(x *uint16) error {
	b := make([]byte, 2)
	if _, err := r.Read(b); err != nil {
		return err
	}
	*x = *(*uint16)(unsafe.Pointer(&b[0]))
	return nil
}

func (r *Reader) BEInt32(x *int32) error {
	b := make([]byte, 4)
	if _, err := r.Read(b); err != nil {
		return err
	}
	*x = int32(binary.BigEndian.Uint32(b))
	return nil
}

func (r *Reader) ByteSlice(x *[]byte) error {
	var length uint32
	r.Varuint32(&length)
	l := int(length)
	int32max := 1<<31 - 1
	if l > int32max {
		return errors.New("byte slice overflows int32")
	}
	data := make([]byte, l)
	if _, err := r.Read(data); err != nil {
		return err
	}
	*x = data
	return nil
}

func (r *Reader) Varuint32(x *uint32) error {
	var v uint32
	for i := 0; i < 35; i += 7 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}

		v |= uint32(b&0x7f) << i
		if b&0x80 == 0 {
			*x = v
			return nil
		}
	}
	return errors.New("varint overflows int32")
}

// Bool reads a bool from the underlying buffer.
func (r *Reader) Bool(x *bool) error {
	u, err := r.ReadByte()
	if err != nil {
		return err
	}

	if u == 0x0 {
		*x = false
	} else if u == 0x1 {
		*x = true
	}

	return nil
}

// Float32 reads a little endian float32 from the underlying buffer.
func (r *Reader) Float32(x *float32) error {
	b := make([]byte, 4)
	if _, err := r.Read(b); err != nil {
		return err
	}
	*x = *(*float32)(unsafe.Pointer(&b[0]))
	return nil
}

// String reads a string from the underlying buffer.
func (r *Reader) String(x *string) error {
	var length uint32
	r.Varuint32(&length)
	l := int(length)
	if l > math.MaxInt32 {
		return errors.New("string to long")
	}
	data := make([]byte, l)
	if _, err := r.Read(data); err != nil {
		return err
	}
	*x = *(*string)(unsafe.Pointer(&data))
	return nil
}
