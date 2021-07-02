package speed

import (
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

func DomainOldWay(r protocol.DecodeReader) string {
	pk, _ := protocol.ReadPacket(r)
	hs, _ := handshaking.UnmarshalServerBoundHandshake(pk)
	return string(hs.ServerAddress)
}

func DomainNewWay(pk protocol.Packet) string {
	return ""
}
