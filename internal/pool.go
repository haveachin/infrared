package internal

import (
	"bytes"
	"sync"
)

// BufferPool is a sync.Pool for buffers used to write compressed data to.
var BufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256))
	},
}
