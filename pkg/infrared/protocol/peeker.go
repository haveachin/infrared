package protocol

import "io"

type PeekReader interface {
	Peek(n int) ([]byte, error)
	io.Reader
}

type BytePeeker struct {
	PeekReader
	Cursor int
}

func (p *BytePeeker) Read(b []byte) (int, error) {
	buf, err := p.Peek(len(b) + p.Cursor)
	if err != nil {
		return 0, err
	}

	for i := 0; i < len(b); i++ {
		b[i] = buf[i+p.Cursor]
	}

	p.Cursor += len(b)

	return len(b), nil
}

func (p *BytePeeker) ReadByte() (byte, error) {
	buf, err := p.Peek(1 + p.Cursor)
	if err != nil {
		return 0x00, err
	}

	b := buf[p.Cursor]
	p.Cursor++

	return b, nil
}
