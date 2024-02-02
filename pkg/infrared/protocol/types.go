package protocol

import (
	"errors"
	"io"

	"github.com/google/uuid"
)

// A Field is both FieldEncoder and FieldDecoder.
type Field interface {
	FieldEncoder
	FieldDecoder
}

// A FieldEncoder can be encoded as minecraft protocol used.
type FieldEncoder io.WriterTo

// A FieldDecoder can Decode from minecraft protocol.
type FieldDecoder io.ReaderFrom

type (
	// Boolean of True is encoded as 0x01, false as 0x00.
	Boolean bool
	// Byte is signed 8-bit integer, two's complement.
	Byte int8
	// UnsignedShort is unsigned 16-bit integer.
	UnsignedShort uint16
	// Long is signed 64-bit integer, two's complement.
	Long int64
	// String is sequence of Unicode scalar values.
	String string

	// Chat is encoded as a String with max length of 32767.
	Chat = String

	// VarInt is variable-length data encoding a two's complement signed 32-bit integer.
	VarInt int32

	// UUID encoded as an unsigned 128-bit integer.
	UUID uuid.UUID

	// ByteArray is []byte with prefix VarInt as length.
	ByteArray []byte
)

const (
	MaxVarIntLen  = 5
	MaxVarLongLen = 10
)

func (b Boolean) WriteTo(w io.Writer) (int64, error) {
	var v byte
	if b {
		v = 0x01
	} else {
		v = 0x00
	}
	nn, err := w.Write([]byte{v})
	return int64(nn), err
}

func (b *Boolean) ReadFrom(r io.Reader) (int64, error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}

	*b = v != 0
	return n, nil
}

func (s String) WriteTo(w io.Writer) (int64, error) {
	byteStr := []byte(s)
	n1, err := VarInt(len(byteStr)).WriteTo(w)
	if err != nil {
		return n1, err
	}
	n2, err := w.Write(byteStr)
	return n1 + int64(n2), err
}

func (s *String) ReadFrom(r io.Reader) (int64, error) {
	var l VarInt // String length

	nn, err := l.ReadFrom(r)
	if err != nil {
		return nn, err
	}
	n := nn

	bs := make([]byte, l)
	if _, err := io.ReadFull(r, bs); err != nil {
		return n, err
	}
	n += int64(l)

	*s = String(bs)
	return n, nil
}

// readByte read one byte from io.Reader.
func readByte(r io.Reader) (int64, byte, error) {
	if r, ok := r.(io.ByteReader); ok {
		v, err := r.ReadByte()
		return 1, v, err
	}
	var v [1]byte
	n, err := r.Read(v[:])
	return int64(n), v[0], err
}

func (b Byte) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write([]byte{byte(b)})
	return int64(nn), err
}

func (b *Byte) ReadFrom(r io.Reader) (int64, error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}
	*b = Byte(v)
	return n, nil
}

func (us UnsignedShort) WriteTo(w io.Writer) (int64, error) {
	n := uint16(us)
	byteLen := uint16(8)
	nn, err := w.Write([]byte{byte(n >> byteLen), byte(n)})
	return int64(nn), err
}

func (us *UnsignedShort) ReadFrom(r io.Reader) (int64, error) {
	var bs [2]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	}
	n := int64(nn)

	*us = UnsignedShort(int16(bs[0])<<8 | int16(bs[1]))
	return n, nil
}

func (l Long) WriteTo(w io.Writer) (int64, error) {
	n := uint64(l)
	nn, err := w.Write([]byte{
		byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	})
	return int64(nn), err
}

func (l *Long) ReadFrom(r io.Reader) (int64, error) {
	var bs [8]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	}
	n := int64(nn)

	*l = Long(int64(bs[0])<<56 | int64(bs[1])<<48 | int64(bs[2])<<40 | int64(bs[3])<<32 |
		int64(bs[4])<<24 | int64(bs[5])<<16 | int64(bs[6])<<8 | int64(bs[7]))
	return n, nil
}

func (v VarInt) WriteTo(w io.Writer) (int64, error) {
	var vi [MaxVarIntLen]byte
	n := v.WriteToBytes(vi[:])
	n, err := w.Write(vi[:n])
	return int64(n), err
}

// WriteToBytes encodes the VarInt into buf and returns the number of bytes written.
// If the buffer is too small, WriteToBytes will panic.
func (v VarInt) WriteToBytes(buf []byte) int {
	num := uint32(v)
	i := 0
	for {
		b := num & 0x7F
		num >>= 7
		if num != 0 {
			b |= 0x80
		}
		buf[i] = byte(b)
		i++
		if num == 0 {
			break
		}
	}
	return i
}

func (v *VarInt) ReadFrom(r io.Reader) (int64, error) {
	var vi uint32
	var num, n int64
	for sec := byte(0x80); sec&0x80 != 0; num++ {
		if num > MaxVarIntLen {
			return 0, errors.New("VarInt is too big")
		}

		var err error
		n, sec, err = readByte(r)
		if err != nil {
			return n, err
		}

		vi |= uint32(sec&0x7F) << uint32(7*num)
	}
	*v = VarInt(vi)
	return n, nil
}

// Len returns the number of bytes required to encode the VarInt.
func (v VarInt) Len() int {
	switch {
	case v < 0:
		return MaxVarIntLen
	case v < 1<<(7*1):
		return 1
	case v < 1<<(7*2):
		return 2
	case v < 1<<(7*3):
		return 3
	case v < 1<<(7*4):
		return 4
	default:
		return 5
	}
}

func (b ByteArray) WriteTo(w io.Writer) (int64, error) {
	n1, err := VarInt(len(b)).WriteTo(w)
	if err != nil {
		return n1, err
	}
	n2, err := w.Write(b)
	return n1 + int64(n2), err
}

func (b *ByteArray) ReadFrom(r io.Reader) (int64, error) {
	var length VarInt
	n1, err := length.ReadFrom(r)
	if err != nil {
		return n1, err
	}
	if cap(*b) < int(length) {
		*b = make(ByteArray, length)
	} else {
		*b = (*b)[:length]
	}
	n2, err := io.ReadFull(r, *b)
	return n1 + int64(n2), err
}

func (u UUID) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write(u[:])
	return int64(nn), err
}

func (u *UUID) ReadFrom(r io.Reader) (int64, error) {
	nn, err := io.ReadFull(r, (*u)[:])
	return int64(nn), err
}
