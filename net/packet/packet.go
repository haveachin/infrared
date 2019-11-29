package packet

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Uncompressed is a packet when it's compression size is 0
const Uncompressed = VarInt(0)

// Packet define a net data package
type Packet struct {
	ID   byte
	Data []byte
}

//Marshal generate Packet with the ID and Fields
func Marshal(ID byte, fields ...FieldEncoder) Packet {
	pk := Packet{
		ID: ID,
	}

	for _, v := range fields {
		pk.Data = append(pk.Data, v.Encode()...)
	}

	return pk
}

//Scan decode the packet and fill data into fields
func (p Packet) Scan(fields ...FieldDecoder) error {
	r := bytes.NewReader(p.Data)
	for _, v := range fields {
		err := v.Decode(r)
		if err != nil {
			return err
		}
	}

	return nil
}

// Pack packs a packet and compresses it when it's size is greater than the threshold
func (p *Packet) Pack(threshold int) []byte {
	data := []byte{p.ID}
	data = append(data, p.Data...)

	var pack []byte

	if threshold > 0 {
		if len(data) > threshold {
			Len := len(data)
			VarLen := VarInt(Len).Encode()
			data = Compress(data)

			pack = append(pack, VarInt(len(VarLen)+len(data)).Encode()...)
			pack = append(pack, VarLen...)
			pack = append(pack, data...)
		} else {
			pack = append(pack, VarInt(int32(len(data)+1)).Encode()...)
			pack = append(pack, 0x00)
			pack = append(pack, data...)
		}
	} else {
		pack = append(pack, VarInt(int32(len(data))).Encode()...)
		pack = append(pack, data...)
	}

	return pack
}

// RecvPacket receive a packet from server
func RecvPacket(r io.ByteReader, useZlib bool) (*Packet, error) {
	var len int
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read len of packet fail: %v", err)
		}
		len |= (int(b&0x7F) << uint(7*i))
		if b&0x80 == 0 {
			break
		}
	}

	if len < 1 {
		return nil, fmt.Errorf("packet length too short")
	}

	data := make([]byte, len)
	var err error
	for i := 0; i < len; i++ {
		data[i], err = r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read content of packet fail: %v", err)
		}
	}

	if useZlib {
		return Unpack(data)
	}

	return &Packet{
		ID:   data[0],
		Data: data[1:],
	}, nil
}

func Unpack(data []byte) (*Packet, error) {
	reader := bytes.NewReader(data)

	var compressionSize VarInt
	if err := compressionSize.Decode(reader); err != nil {
		return nil, err
	}

	buffer := make([]byte, compressionSize)
	if compressionSize != Uncompressed {
		r, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("decompress fail: %v", err)
		}

		_, err = io.ReadFull(r, buffer)
		if err != nil {
			return nil, fmt.Errorf("decompress fail: %v", err)
		}

		r.Close()
	} else {
		buffer = data[1:]
	}

	return &Packet{
		ID:   buffer[0],
		Data: buffer[1:],
	}, nil
}

func Compress(data []byte) []byte {
	var buffer bytes.Buffer

	w := zlib.NewWriter(&buffer)
	w.Write(data)
	w.Close()

	return buffer.Bytes()
}
