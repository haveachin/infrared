package protocol

import (
	"io"
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
