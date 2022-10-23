package packet

import (
	"bytes"
	"fmt"
	"io"

	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
)

// Packet represents a packet that may be sent over a Minecraft network connection. The packet needs to hold
// a method to encode itself to binary and decode itself from binary.
type Packet interface {
	// ID returns the ID of the packet. All of these identifiers of packets may be found in id.go.
	ID() uint32
	// Unmarshal decodes a serialised packet in buf into the Packet instance. The serialised packet passed
	// into Unmarshal will not have a header in it.
	Unmarshal(r *protocol.Reader) error
	Marshal(w *protocol.Writer)
}

// Header is the header of a packet. It exists out of a single varuint32 which is composed of a packet ID and
// a sender and target sub client ID. These IDs are used for split screen functionality.
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

type Data struct {
	Header  Header
	Full    []byte
	Payload *bytes.Buffer
}

// ParseData parses the packet data slice passed into a Data struct.
func ParseData(bb []byte) (Data, error) {
	buf := bytes.NewBuffer(bb)
	header := Header{}
	if err := header.Read(buf); err != nil {
		return Data{}, fmt.Errorf("error reading packet header: %v", err)
	}
	return Data{Header: header, Full: bb, Payload: buf}, nil
}

// decode decodes the packet payload held in the packetData and returns the packet.Packet decoded.
func (d *Data) Decode(pk Packet) error {
	if err := pk.Unmarshal(protocol.NewReader(d.Payload)); err != nil {
		return err
	}
	if d.Payload.Len() != 0 {
		return fmt.Errorf("%T: %v unread bytes left: 0x%x", pk, d.Payload.Len(), d.Payload.Bytes())
	}
	return nil
}
