package protocol

import (
	"bytes"
	"fmt"
	"io"
)

// Packet is the raw representation of message that is send between the client and the server
type Packet struct {
	ID   byte
	Data []byte
}

// Scan decodes and copies the Packet data into the fields
func (pk Packet) Scan(fields ...FieldDecoder) error {
	return ScanFields(bytes.NewReader(pk.Data), fields...)
}

// Marshal encodes the packet and all it's fields
func (pk *Packet) Marshal() ([]byte, error) {
	var packedData []byte
	data := []byte{pk.ID}
	data = append(data, pk.Data...)

	packedData = append(packedData, VarInt(int32(len(data))).Encode()...)

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

// MarshalPacket transforms an ID and Fields into a Packet
func MarshalPacket(ID byte, fields ...FieldEncoder) Packet {
	var pkt Packet
	pkt.ID = ID

	for _, v := range fields {
		pkt.Data = append(pkt.Data, v.Encode()...)
	}

	return pkt
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
func ReadPacket(r DecodeReader) (Packet, error) {
	data, err := ReadPacketBytes(r)
	if err != nil {
		return Packet{}, err
	}

	return Packet{
		ID:   data[0],
		Data: data[1:],
	}, nil
}

// PeekPacket decodes and decompresses a byte stream and peeks the first Packet
func PeekPacket(p PeekReader) (Packet, error) {
	r := bytePeeker{
		PeekReader: p,
		cursor:     0,
	}

	return ReadPacket(&r)
}
