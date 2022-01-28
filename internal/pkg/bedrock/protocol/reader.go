package protocol

import (
	"encoding/binary"
	"errors"
	"io"
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
