package main

import (
	"bytes"
	"log"
	"net"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

var payload []byte

func init() {
	buf := bytes.NewBuffer([]byte{})
	buf.Write(protocol.VarInt(2147483647).Encode())
	payload = buf.Bytes()
}

func main() {
	for i := 0; i < 30; i++ {
		c, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
		}

		c.Write(payload)
	}
}
