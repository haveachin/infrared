package protocol

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// A Packet is the raw representation of message that is send between the client and the server
type Packet struct {
	ID   byte
	Data []byte
}

// ScanFields decodes and copies the Packet data into the fields
func (pk Packet) Scan(fields ...FieldDecoder) error {
	return ScanFields(bytes.NewReader(pk.Data), fields...)
}

// MarshalPacket encodes the packet and compresses it if it is larger then the given threshold
func (pk *Packet) Marshal(threshold int) ([]byte, error) {
	var packedData []byte
	data := []byte{pk.ID}
	data = append(data, pk.Data...)

	if threshold > 0 {
		if len(data) > threshold {
			length := VarInt(len(data)).Encode()

			var b bytes.Buffer
			w := zlib.NewWriter(&b)
			if _, err := w.Write(data); err != nil {
				return nil, err
			}
			_ = w.Close()
			data = b.Bytes()

			packedData = append(packedData, VarInt(len(length)+len(data)).Encode()...)
			packedData = append(packedData, length...)
		} else {
			packedData = append(packedData, VarInt(int32(len(data)+1)).Encode()...)
			packedData = append(packedData, 0x00)
		}
	} else {
		packedData = append(packedData, VarInt(int32(len(data))).Encode()...)
	}

	return append(packedData, data...), nil
}

// ScanFields decodes a byte stream into fields
func ScanFields(r DecodeReader, fields ...FieldDecoder) error {
	for _, field := range fields {
		if err := field.Decode(r); err != nil {
			return err
		}
	}
	return nil
}

func MarshalFields(fields ...FieldEncoder) []byte {
	var b []byte
	for _, field := range fields {
		b = append(b, field.Encode()...)
	}
	return b
}

// MarshalPacket transforms an ID and Fields into a Packet
func MarshalPacket(ID byte, fields ...FieldEncoder) Packet {
	var pkt Packet
	pkt.ID = ID

	for _, v := range fields {
		pkt.Data = append(pkt.Data, v.Encode()...)
	}

	return pkt
}

// ParsePacket decodes and decompresses a byte array into a Packet
func ParsePacket(data []byte) (Packet, error) {
	reader := bytes.NewBuffer(data)

	var dataLength VarInt
	if err := dataLength.Decode(reader); err != nil {
		return Packet{}, err
	}

	decompressedData := make([]byte, dataLength)
	isCompressed := dataLength != 0
	if isCompressed {
		r, err := zlib.NewReader(reader)
		if err != nil {
			return Packet{}, err
		}
		defer r.Close()
		_, err = io.ReadFull(r, decompressedData)
		if err != nil {
			return Packet{}, err
		}
	} else {
		decompressedData = data[1:]
	}

	return Packet{
		ID:   decompressedData[0],
		Data: decompressedData[1:],
	}, nil
}

// ReadPacketBytes decodes a byte stream and cuts the first Packet as a byte array out
func ReadPacketBytes(r DecodeReader) ([]byte, error) {
	var packetLength VarInt
	if err := packetLength.Decode(r); err != nil {
		return nil, err
	}

	if packetLength < 1 {
		return nil, fmt.Errorf("packet length too short")
	}

	data := make([]byte, packetLength)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("reading the content of the packet failed: %v", err)
	}

	return data, nil
}

// ReadPacket decodes and decompresses a byte stream and cuts the first Packet out
func ReadPacket(r DecodeReader, isZlib bool) (Packet, error) {
	data, err := ReadPacketBytes(r)
	if err != nil {
		return Packet{}, err
	}

	if isZlib {
		return ParsePacket(data)
	}

	return Packet{
		ID:   data[0],
		Data: data[1:],
	}, nil
}

// PeekPacket decodes and decompresses a byte stream and peeks the first Packet
func PeekPacket(p PeekReader, isZlib bool) (Packet, error) {
	r := bytePeeker{
		PeekReader: p,
		cursor:     0,
	}

	return ReadPacket(&r, isZlib)
}
