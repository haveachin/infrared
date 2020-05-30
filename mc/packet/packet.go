package packet

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Packet define a net data package
type Packet struct {
	ID   byte
	Data []byte
}

//Marshal generate Packet with the ID and Fields
func Marshal(ID byte, fields ...FieldEncoder) (pk Packet) {
	pk.ID = ID

	for _, v := range fields {
		pk.Data = append(pk.Data, v.Encode()...)
	}

	return
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

// Pack packs the packet and compresses it if it is larger then the given threshold
func (p *Packet) Pack(threshold int) []byte {
	var packedData []byte
	data := []byte{p.ID}
	data = append(data, p.Data...)

	if threshold > 0 {
		if len(data) > threshold {
			length := VarInt(len(data)).Encode()
			data = Compress(data)

			packedData = append(packedData, VarInt(len(length)+len(data)).Encode()...)
			packedData = append(packedData, length...)
		} else {
			packedData = append(packedData, VarInt(int32(len(data)+1)).Encode()...)
			packedData = append(packedData, 0x00)
		}
	} else {
		packedData = append(packedData, VarInt(int32(len(data))).Encode()...)
	}

	return append(packedData, data...)
}

// RecvPacket receive a packet from server
func Read(r io.ByteReader, zlib bool) (Packet, error) {
	var length int
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return Packet{}, fmt.Errorf("read length of packet fail: %v", err)
		}

		length |= int(b&0x7F) << uint(7*i)
		if b&0x80 == 0 {
			break
		}
	}

	if length < 1 {
		return Packet{}, fmt.Errorf("packet length too short")
	}

	data := make([]byte, length)
	var err error
	for i := 0; i < length; i++ {
		data[i], err = r.ReadByte()
		if err != nil {
			return Packet{}, fmt.Errorf("read content of packet fail: %v", err)
		}
	}

	if zlib {
		return Decompress(data)
	}

	return Packet{
		ID:   data[0],
		Data: data[1:],
	}, nil
}

func Peek(p Peeker, zlib bool) (Packet, error) {
	r := bytePeeker{
		Peeker: p,
		cursor: 0,
	}

	return Read(&r, zlib)
}

// Decompress 读取一个压缩的包
func Decompress(data []byte) (Packet, error) {
	reader := bytes.NewReader(data)

	var sizeUncompressed VarInt
	if err := sizeUncompressed.Decode(reader); err != nil {
		return Packet{}, err
	}

	decompressedData := make([]byte, sizeUncompressed)
	if sizeUncompressed != 0 { // != 0 means compressed, let's decompress
		r, err := zlib.NewReader(reader)

		if err != nil {
			return Packet{}, fmt.Errorf("decompress fail: %v", err)
		}
		_, err = io.ReadFull(r, decompressedData)
		if err != nil {
			return Packet{}, fmt.Errorf("decompress fail: %v", err)
		}
		r.Close()
	} else {
		decompressedData = data[1:]
	}
	return Packet{
		ID:   decompressedData[0],
		Data: decompressedData[1:],
	}, nil
}

// Compress 压缩数据
func Compress(data []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(data)
	w.Close()
	return b.Bytes()
}