package main

import (
	"log"
	"net"

	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
)

var handshakePayload []byte

func init() {
	handshake := handshaking.ServerBoundHandshake{
		ProtocolVersion: 758,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       1,
	}

	pk := handshake.Marshal()

	bb, err := pk.Marshal()
	if err != nil {
		log.Fatal(err)
	}

	handshakePayload = bb
}

func main() {
	for i := 0; i < 10000000; i++ {
		c, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
		}

		c.Write(handshakePayload)
	}
}
