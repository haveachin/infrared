package protocol

import "io"

type PeekReader interface {
	Peek(n int) ([]byte, error)
	io.Reader
}

type bytePeeker struct {
	PeekReader
	cursor int
}

func (peeker *bytePeeker) Read(b []byte) (int, error) {
	buf, err := peeker.Peek(len(b) + peeker.cursor)
	if err != nil {
		return 0, err
	}

	for i := 0; i < len(b); i++ {
		b[i] = buf[i+peeker.cursor]
	}

	peeker.cursor += len(b)

	return len(buf), nil
}

func (peeker *bytePeeker) ReadByte() (byte, error) {
	buf, err := peeker.Peek(1 + peeker.cursor)
	if err != nil {
		return 0x00, err
	}

	b := buf[peeker.cursor]
	peeker.cursor++

	return b, nil
}
