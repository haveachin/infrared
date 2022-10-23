package packet

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/golang/snappy"
	"github.com/haveachin/infrared/internal"
	"github.com/klauspost/compress/flate"
)

// Compression represents a compression algorithm that can compress and decompress data.
type Compression interface {
	// EncodeCompression encodes the compression algorithm into a uint16 ID.
	EncodeCompression() uint16
	// Compress compresses the given data and returns the compressed data.
	Compress(decompressed []byte) ([]byte, error)
	// Decompress decompresses the given data and returns the decompressed data.
	Decompress(compressed []byte) ([]byte, error)
}

type (
	// FlateCompression is the implementation of the Flate compression algorithm. This was used by default until v1.19.30.
	FlateCompression struct{}
	// SnappyCompression is the implementation of the Snappy compression algorithm. This is used by default.
	SnappyCompression struct{}
)

// flateDecompressPool is a sync.Pool for io.ReadCloser flate readers. These are
// pooled for connections.
var (
	flateDecompressPool = sync.Pool{
		New: func() any { return flate.NewReader(bytes.NewReader(nil)) },
	}
	flateCompressPool = sync.Pool{
		New: func() any {
			w, _ := flate.NewWriter(io.Discard, 6)
			return w
		},
	}
)

// EncodeCompression ...
func (FlateCompression) EncodeCompression() uint16 {
	return 0
}

// Compress ...
func (FlateCompression) Compress(decompressed []byte) ([]byte, error) {
	compressed := internal.BufferPool.Get().(*bytes.Buffer)
	w := flateCompressPool.Get().(*flate.Writer)

	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		compressed.Reset()
		internal.BufferPool.Put(compressed)
		flateCompressPool.Put(w)
	}()

	w.Reset(compressed)

	_, err := w.Write(decompressed)
	if err != nil {
		return nil, fmt.Errorf("compress flate: %w", err)
	}
	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("close flate writer: %w", err)
	}
	return compressed.Bytes(), nil
}

// Decompress ...
func (FlateCompression) Decompress(compressed []byte) ([]byte, error) {
	buf := bytes.NewReader(compressed)
	c := flateDecompressPool.Get().(io.ReadCloser)
	defer flateDecompressPool.Put(c)

	if err := c.(flate.Resetter).Reset(buf, nil); err != nil {
		return nil, fmt.Errorf("reset flate: %w", err)
	}
	_ = c.Close()

	// Guess an uncompressed size of 2*len(compressed).
	decompressed := bytes.NewBuffer(make([]byte, 0, len(compressed)*2))
	if _, err := io.Copy(decompressed, c); err != nil {
		return nil, fmt.Errorf("decompress flate: %v", err)
	}
	return decompressed.Bytes(), nil
}

// EncodeCompression ...
func (SnappyCompression) EncodeCompression() uint16 {
	return 1
}

// Compress ...
func (SnappyCompression) Compress(decompressed []byte) ([]byte, error) {
	// Because Snappy allocates a slice only once, it is less important to have
	// a dst slice pre-allocated. With FlateCompression this is more important,
	// because flate does a lot of smaller allocations which causes a
	// considerable slowdown.
	return snappy.Encode(nil, decompressed), nil
}

// Decompress ...
func (SnappyCompression) Decompress(compressed []byte) ([]byte, error) {
	// Snappy writes a decoded data length prefix, so it can allocate the
	// perfect size right away and only needs to allocate once. No need to pool
	// byte slices here either.
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress snappy: %w", err)
	}
	return decompressed, nil
}

// init registers all valid compressions with the protocol.
func init() {
	RegisterCompression(FlateCompression{}, "flate")
	RegisterCompression(SnappyCompression{}, "snappy")
}

var compressions = map[uint16]Compression{}
var compressionsByName = map[string]Compression{}

// RegisterCompression registers a compression so that it can be used by the protocol. Name is case insensitive.
func RegisterCompression(compression Compression, name string) {
	compressions[compression.EncodeCompression()] = compression
	compressionsByName[strings.ToLower(name)] = compression
}

// CompressionByID attempts to return a compression by the ID it was registered with. If found, the compression found
// is returned and the bool is true.
func CompressionByID(id uint16) (Compression, bool) {
	c, ok := compressions[id]
	return c, ok
}

// CompressionByName attempts to return a compression by the name it was registered with. If found, the compression found
// is returned and the bool is true. Name is case insensitive.
func CompressionByName(name string) (Compression, bool) {
	c, ok := compressionsByName[strings.ToLower(name)]
	return c, ok
}
