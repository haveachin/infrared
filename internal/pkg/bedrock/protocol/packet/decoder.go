package packet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	// header is the header of compressed 'batches' from Minecraft.
	header = 0xfe
	// maximumInBatch is the maximum amount of packets that may be found in a batch. If a compressed batch has
	// more than this amount, decoding will fail.
	maximumInBatch = 512 + 256
)

// Decoder handles the decoding of Minecraft packets sent through an io.Reader. These packets in turn contain
// multiple compressed packets.
type Decoder struct {
	// r holds the io.Reader that packets are read from if the reader does not implement packetReader. When
	// this is the case, the buf field has a non-zero length.
	r   io.Reader
	buf []byte

	// pr holds a packetReader (and io.Reader) that packets are read from if the io.Reader passed to
	// NewDecoder implements the packetReader interface.
	pr packetReader

	compression Compression

	checkPacketLimit bool
}

// packetReader is used to read packets immediately instead of copying them in a buffer first. This is a
// specific case made to reduce RAM usage.
type packetReader interface {
	ReadPacket() ([]byte, error)
}

// NewDecoder returns a new decoder decoding data from the io.Reader passed. One read call from the reader is
// assumed to consume an entire packet.
func NewDecoder(reader io.Reader) *Decoder {
	if pr, ok := reader.(packetReader); ok {
		return &Decoder{checkPacketLimit: true, pr: pr}
	}
	return &Decoder{
		r:                reader,
		buf:              make([]byte, 1024*1024*3),
		checkPacketLimit: true,
	}
}

// EnableCompression enables compression for the Decoder.
func (decoder *Decoder) EnableCompression(compression Compression) {
	decoder.compression = compression
}

// DisableBatchPacketLimit disables the check that limits the number of packets allowed in a single packet
// batch. This should typically be called for Decoders decoding from a server connection.
func (decoder *Decoder) DisableBatchPacketLimit() {
	decoder.checkPacketLimit = false
}

// Decode decodes one 'packet' from the io.Reader passed in NewDecoder(), producing a slice of packets that it
// held and an error if not successful.
func (decoder *Decoder) Decode() (packets [][]byte, err error) {
	var data []byte
	if decoder.pr == nil {
		var n int
		n, err = decoder.r.Read(decoder.buf)
		data = decoder.buf[:n]
	} else {
		data, err = decoder.pr.ReadPacket()
	}
	if err != nil {
		return nil, fmt.Errorf("error reading batch from reader: %v", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	if data[0] != header {
		return nil, fmt.Errorf("error reading packet: invalid packet header %x: expected %x", data[0], header)
	}
	data = data[1:]

	if decoder.compression != nil {
		data, err = decoder.compression.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("error decompressing packet: %v", err)
		}
	}

	b := bytes.NewBuffer(data)
	for b.Len() != 0 {
		var length uint32
		if err := Varuint32(b, &length); err != nil {
			return nil, fmt.Errorf("error reading packet length: %v", err)
		}
		packets = append(packets, b.Next(int(length)))
	}
	if len(packets) > maximumInBatch && decoder.checkPacketLimit {
		return nil, fmt.Errorf("number of packets %v in compressed batch exceeds %v", len(packets), maximumInBatch)
	}
	return packets, nil
}

func Varuint32(src io.ByteReader, x *uint32) error {
	var v uint32
	for i := uint(0); i < 35; i += 7 {
		b, err := src.ReadByte()
		if err != nil {
			return err
		}
		v |= uint32(b&0x7f) << i
		if b&0x80 == 0 {
			*x = v
			return nil
		}
	}
	return errors.New("varuint32 did not terminate after 5 bytes")
}
