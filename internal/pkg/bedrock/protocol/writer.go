package protocol

import (
	"io"
	"unsafe"
)

type EncodeReader interface {
	io.Writer
	io.ByteWriter
}

type Writer struct {
	EncodeReader
}

func NewWriter(w EncodeReader) *Writer {
	return &Writer{EncodeReader: w}
}

func (w *Writer) Bool(x bool) {
	if x {
		w.WriteByte(0x01)
	} else {
		w.WriteByte(0x00)
	}
}

func (w *Writer) String(x string) {
	l := uint32(len(x))
	w.Varuint32(l)
	_, _ = w.Write([]byte(x))
}

func (w *Writer) Varuint32(x uint32) {
	for x >= 0x80 {
		_ = w.WriteByte(byte(x) | 0x80)
		x >>= 7
	}
	_ = w.WriteByte(byte(x))
}

// Uint8 writes a uint8 to the underlying buffer.
func (w *Writer) Uint8(x uint8) {
	_ = w.WriteByte(x)
}

// Uint16 writes a little endian uint16 to the underlying buffer.
func (w *Writer) Uint16(x uint16) {
	data := *(*[2]byte)(unsafe.Pointer(&x))
	_, _ = w.Write(data[:])
}

// Float32 writes a little endian float32 to the underlying buffer.
func (w *Writer) Float32(x float32) {
	data := *(*[4]byte)(unsafe.Pointer(&x))
	_, _ = w.Write(data[:])
}

// BEInt32 writes a big endian int32 to the underlying buffer.
func (w *Writer) BEInt32(x *int32) {
	data := *(*[4]byte)(unsafe.Pointer(x))
	_, _ = w.Write(data[:])
}

// ByteSlice writes a []byte, prefixed with a varuint32, to the underlying buffer.
func (w *Writer) ByteSlice(x *[]byte) {
	w.Varuint32(uint32(len(*x)))
	_, _ = w.Write(*x)
}
