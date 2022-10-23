package packet

import (
	"bytes"
	"fmt"
	"io"

	"github.com/haveachin/infrared/internal"
)

// Encoder handles the encoding of Minecraft packets that are sent to an io.Writer. The packets are compressed
// and optionally encoded before they are sent to the io.Writer.
type Encoder struct {
	w io.Writer

	compression Compression
}

// NewEncoder returns a new Encoder for the io.Writer passed. Each final packet produced by the Encoder is
// sent with a single call to io.Writer.Write().
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

// EnableCompression enables compression for the Encoder.
func (encoder *Encoder) EnableCompression(compression Compression) {
	encoder.compression = compression
}

// Encode encodes the packets passed. It writes all of them as a single packet which is  compressed and
// optionally encrypted.
func (encoder *Encoder) Encode(packet []byte) error {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	l := make([]byte, 5)
	// Each packet is prefixed with a varuint32 specifying the length of the packet.
	if err := writeVaruint32(buf, uint32(len(packet)), l); err != nil {
		return fmt.Errorf("error writing varuint32 length: %v", err)
	}
	if _, err := buf.Write(packet); err != nil {
		return fmt.Errorf("error writing packet payload: %v", err)
	}

	data := buf.Bytes()
	if encoder.compression != nil {
		var err error
		data, err = encoder.compression.Compress(data)
		if err != nil {
			return fmt.Errorf("error compressing packet: %v", err)
		}
	}

	data = append([]byte{header}, data...)
	if _, err := encoder.w.Write(data); err != nil {
		return fmt.Errorf("error writing compressed packet to io.Writer: %v", err)
	}
	return nil
}

// writeVaruint32 writes a uint32 to the destination buffer passed with a size of 1-5 bytes. It uses byte
// slice b in order to prevent allocations.
func writeVaruint32(dst io.Writer, x uint32, b []byte) error {
	b[4] = 0
	b[3] = 0
	b[2] = 0
	b[1] = 0
	b[0] = 0

	i := 0
	for x >= 0x80 {
		b[i] = byte(x) | 0x80
		i++
		x >>= 7
	}
	b[i] = byte(x)
	_, err := dst.Write(b[:i+1])
	return err
}
