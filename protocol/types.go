package protocol

import (
	"bytes"
	"errors"
	"github.com/gofrs/uuid"
	"io"
)

// A Field is both FieldEncoder and FieldDecoder
type Field interface {
	FieldEncoder
	FieldDecoder
}

// A FieldEncoder can be encode as minecraft protocol used.
type FieldEncoder interface {
	Encode() []byte
}

// A FieldDecoder can Decode from minecraft protocol
type FieldDecoder interface {
	Decode(r DecodeReader) error
}

//DecodeReader is both io.Reader and io.ByteReader
type DecodeReader interface {
	io.ByteReader
	io.Reader
}

type (
	// Boolean of True is encoded as 0x01, false as 0x00.
	Boolean bool
	// Byte is signed 8-bit integer, two's complement
	Byte int8
	// UnsignedShort is unsigned 16-bit integer
	UnsignedShort uint16
	// Long is signed 64-bit integer, two's complement
	Long int64
	// String is sequence of Unicode scalar values
	String string

	// Chat is encoded as a String with max length of 32767.
	Chat = String
	// Identifier is encoded as a String with max length of 32767.
	Identifier = String

	// VarInt is variable-length data encoding a two's complement signed 32-bit integer
	VarInt int32

	// UUID encoded as an unsigned 128-bit integer
	UUID uuid.UUID

	// ByteArray is []byte with prefix VarInt as length
	ByteArray []byte

	// OptionalByteArray is []byte without prefix VarInt as length
	OptionalByteArray []byte
)

// ReadNBytes read N bytes from bytes.Reader
func ReadNBytes(r DecodeReader, n int) ([]byte, error) {
	bb := make([]byte, n)
	var err error
	for i := 0; i < n; i++ {
		bb[i], err = r.ReadByte()
		if err != nil {
			return nil, err
		}
	}
	return bb, nil
}

// Encode a Boolean
func (b Boolean) Encode() []byte {
	if b {
		return []byte{0x01}
	}
	return []byte{0x00}
}

// Decode a Boolean
func (b *Boolean) Decode(r DecodeReader) error {
	v, err := r.ReadByte()
	if err != nil {
		return err
	}

	*b = v != 0
	return nil
}

// Encode a String
func (s String) Encode() []byte {
	byteString := []byte(s)
	var bb []byte
	bb = append(bb, VarInt(len(byteString)).Encode()...) // len
	bb = append(bb, byteString...)                       // data
	return bb
}

// Decode a String
func (s *String) Decode(r DecodeReader) error {
	var l VarInt // String length
	if err := l.Decode(r); err != nil {
		return err
	}

	bb, err := ReadNBytes(r, int(l))
	if err != nil {
		return err
	}

	*s = String(bb)
	return nil
}

// Encode a Byte
func (b Byte) Encode() []byte {
	return []byte{byte(b)}
}

// Decode a Byte
func (b *Byte) Decode(r DecodeReader) error {
	v, err := r.ReadByte()
	if err != nil {
		return err
	}
	*b = Byte(v)
	return nil
}

// Encode a Unsigned Short
func (us UnsignedShort) Encode() []byte {
	n := uint16(us)
	return []byte{
		byte(n >> 8),
		byte(n),
	}
}

// Decode a UnsignedShort
func (us *UnsignedShort) Decode(r DecodeReader) error {
	bb, err := ReadNBytes(r, 2)
	if err != nil {
		return err
	}

	*us = UnsignedShort(int16(bb[0])<<8 | int16(bb[1]))
	return nil
}

// Encode a Long
func (l Long) Encode() []byte {
	n := uint64(l)
	return []byte{
		byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	}
}

// Decode a Long
func (l *Long) Decode(r DecodeReader) error {
	bb, err := ReadNBytes(r, 8)
	if err != nil {
		return err
	}

	*l = Long(int64(bb[0])<<56 | int64(bb[1])<<48 | int64(bb[2])<<40 | int64(bb[3])<<32 |
		int64(bb[4])<<24 | int64(bb[5])<<16 | int64(bb[6])<<8 | int64(bb[7]))
	return nil
}

// Encode a VarInt
func (v VarInt) Encode() []byte {
	num := uint32(v)
	var bb []byte
	for {
		b := num & 0x7F
		num >>= 7
		if num != 0 {
			b |= 0x80
		}
		bb = append(bb, byte(b))
		if num == 0 {
			break
		}
	}
	return bb
}

// Decode a VarInt
func (v *VarInt) Decode(r DecodeReader) error {
	var n uint32
	for i := 0; ; i++ {
		sec, err := r.ReadByte()
		if err != nil {
			return err
		}

		n |= uint32(sec&0x7F) << uint32(7*i)

		if i >= 5 {
			return errors.New("VarInt is too big")
		} else if sec&0x80 == 0 {
			break
		}
	}

	*v = VarInt(n)
	return nil
}

// Encode a ByteArray
func (b ByteArray) Encode() []byte {
	return append(VarInt(len(b)).Encode(), b...)
}

// Decode a ByteArray
func (b *ByteArray) Decode(r DecodeReader) error {
	var length VarInt
	if err := length.Decode(r); err != nil {
		return err
	}
	*b = make([]byte, length)
	_, err := r.Read(*b)
	return err
}

// Encode a UUID
func (u UUID) Encode() []byte {
	return u[:]
}

// Decode a UUID
func (u *UUID) Decode(r DecodeReader) error {
	_, err := io.ReadFull(r, (*u)[:])
	return err
}

// Encode a OptionalByteArray
func (b OptionalByteArray) Encode() []byte {
	return b
}

// Decode a OptionalByteArray
func (b *OptionalByteArray) Decode(r DecodeReader) error {
	buf := bytes.NewBuffer([]byte{})
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}
	*b = buf.Bytes()
	return nil
}
