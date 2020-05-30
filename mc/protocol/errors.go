package protocol

import "errors"

var (
	ErrInvalidPacketID = errors.New("invalid packet id")
	ErrNotAPointer = errors.New("packet is not a pointer")
)
