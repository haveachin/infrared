package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type Header struct {
	PacketID        uint32
	SenderSubClient byte
	TargetSubClient byte
}

func (header *Header) Write(w io.ByteWriter) error {
	x := header.PacketID | (uint32(header.SenderSubClient) << 10) | (uint32(header.TargetSubClient) << 12)
	for x >= 0x80 {
		if err := w.WriteByte(byte(x) | 0x80); err != nil {
			return err
		}
		x >>= 7
	}
	return w.WriteByte(byte(x))
}

func (header *Header) Read(r io.ByteReader) error {
	var value uint32
	if err := Varuint32(r, &value); err != nil {
		return err
	}
	header.PacketID = value & 0x3FF
	header.SenderSubClient = byte((value >> 10) & 0x3)
	header.TargetSubClient = byte((value >> 12) & 0x3)
	return nil
}

func UnmarshalPacket(b []byte, pk Packet) error {
	data, err := parseData(b)
	if err != nil {
		return err
	}

	if data.h.PacketID != pk.ID() {
		return fmt.Errorf("invalid id: 0x%x", data.h.PacketID)
	}

	return data.decode(pk)
}

func MarshalPacket(pk Packet) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	w := NewWriter(buf)

	header := Header{PacketID: pk.ID()}
	_ = header.Write(w)
	pk.Marshal(w)

	encodedPk := bytes.NewBuffer([]byte{})
	encoder := NewEncoder(encodedPk)
	if err := encoder.Encode(buf.Bytes()); err != nil {
		return nil, err
	}

	return encodedPk.Bytes(), nil
}

type packetData struct {
	h       *Header
	full    []byte
	payload *bytes.Buffer
}

// parseData parses the packet data slice passed into a packetData struct.
func parseData(data []byte) (*packetData, error) {
	buf := bytes.NewBuffer(data)
	header := &Header{}
	if err := header.Read(buf); err != nil {
		// We don't return this as an error as it's not in the hand of the user to control this. Instead,
		// we return to reading a new packet.
		return nil, fmt.Errorf("error reading packet header: %v", err)
	}
	return &packetData{h: header, full: data, payload: buf}, nil
}

// Packet represents a packet that may be sent over a Minecraft network connection. The packet needs to hold
// a method to encode itself to binary and decode itself from binary.
type Packet interface {
	// ID returns the ID of the packet. All of these identifiers of packets may be found in id.go.
	ID() uint32
	// Unmarshal decodes a serialised packet in buf into the Packet instance. The serialised packet passed
	// into Unmarshal will not have a header in it.
	Unmarshal(r *Reader) error
	Marshal(w *Writer)
}

// decode decodes the packet payload held in the packetData and returns the packet.Packet decoded.
func (p *packetData) decode(pk Packet) error {
	if err := pk.Unmarshal(NewReader(p.payload)); err != nil {
		return err
	}
	if p.payload.Len() != 0 {
		return fmt.Errorf("%T: %v unread bytes left: 0x%x", pk, p.payload.Len(), p.payload.Bytes())
	}
	return nil
}
