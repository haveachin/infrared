package protocol

import (
	"bytes"
	"fmt"
	"io"
)

const MaxDataLength = 0x200000

type Packet struct {
	ID   int32
	Data []byte
}

func (pk Packet) Decode(fields ...FieldDecoder) error {
	r := bytes.NewReader(pk.Data)
	return ScanFields(r, fields...)
}

func ScanFields(r io.Reader, fields ...FieldDecoder) error {
	for i, v := range fields {
		_, err := v.ReadFrom(r)
		if err != nil {
			return fmt.Errorf("scanning packet field[%d] error: %w", i, err)
		}
	}
	return nil
}

func (pk *Packet) Encode(id int32, fields ...FieldEncoder) {
	buf := bytes.NewBuffer(pk.Data[:0])
	for _, f := range fields {
		f.WriteTo(buf)
	}
	pk.ID = id
	pk.Data = buf.Bytes()
}

func (pk Packet) WriteTo(w io.Writer) (int64, error) {
	pkLen := VarInt(VarInt(pk.ID).Len() + len(pk.Data))
	nLen, err := pkLen.WriteTo(w)
	if err != nil {
		return nLen, err
	}
	n := nLen

	nID, err := VarInt(pk.ID).WriteTo(w)
	n += nID
	if err != nil {
		return n, err
	}

	nData, err := w.Write(pk.Data)
	n += int64(nData)
	if err != nil {
		return n, err
	}

	return n, err
}

func (pk *Packet) ReadFrom(r io.Reader) (int64, error) {
	var pkLen VarInt
	nLen, err := pkLen.ReadFrom(r)
	if err != nil {
		return nLen, err
	}
	n := nLen

	var pkID VarInt
	nID, err := pkID.ReadFrom(r)
	n += nID
	if err != nil {
		return n, err
	}
	pk.ID = int32(pkID)

	lengthOfData := int(pkLen) - int(nID)
	if lengthOfData < 0 || lengthOfData > MaxDataLength {
		return n, fmt.Errorf("invalid packet data length of %d", lengthOfData)
	}

	if cap(pk.Data) < lengthOfData {
		pk.Data = make([]byte, lengthOfData)
	} else {
		pk.Data = pk.Data[:lengthOfData]
	}

	nData, err := io.ReadFull(r, pk.Data)
	n += int64(nData)
	if err != nil {
		return n, err
	}

	return n, nil
}

type Builder struct {
	buf bytes.Buffer
}

func (p *Builder) WriteField(fields ...FieldEncoder) {
	for _, f := range fields {
		_, err := f.WriteTo(&p.buf)
		if err != nil {
			panic(err)
		}
	}
}

func (p *Builder) Packet(id int32) Packet {
	return Packet{ID: id, Data: p.buf.Bytes()}
}
