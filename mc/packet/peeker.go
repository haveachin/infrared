package packet

type Peeker interface {
	Peek(n int) ([]byte, error)
}

type bytePeeker struct {
	Peeker
	cursor int
}

func (p *bytePeeker) ReadByte() (byte, error) {
	buf, err := p.Peek(1 + p.cursor)
	if err != nil {
		return 0x00, err
	}

	b := buf[p.cursor]
	p.cursor++

	return b, nil
}