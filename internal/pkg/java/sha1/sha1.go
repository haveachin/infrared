package sha1

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"strings"
)

type Hash struct {
	hash.Hash
}

func NewHash() Hash {
	return Hash{
		Hash: sha1.New(),
	}
}

func (h Hash) Update(b []byte) {
	// This will never return an error like documented in hash.Hash
	// so ignoring this error is ok
	_, _ = h.Write(b)
}

func (h Hash) HexDigest() string {
	hashBytes := h.Sum(nil)

	negative := (hashBytes[0] & 0x80) == 0x80
	if negative {
		// two's compliment, big endian
		carry := true
		for i := len(hashBytes) - 1; i >= 0; i-- {
			hashBytes[i] = ^hashBytes[i]
			if carry {
				carry = hashBytes[i] == 0xff
				hashBytes[i]++
			}
		}
	}

	hashString := strings.TrimLeft(fmt.Sprintf("%x", hashBytes), "0")
	if negative {
		hashString = "-" + hashString
	}

	return hashString
}
