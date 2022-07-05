package main

import (
	"log"
	"net"

	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
)

var handshakePayload []byte
var loginStartPayload []byte

func init() {
	handshake := handshaking.ServerBoundHandshake{
		ProtocolVersion: 758,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       2,
	}
	pk := handshake.Marshal()
	bb, err := pk.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	handshakePayload = bb

	loginStart := login.ServerLoginStart{
		Name:         "Test",
		HasPublicKey: false,
	}
	pk = loginStart.Marshal()
	bb, err = pk.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	loginStartPayload = bb
}

func main() {
	for i := 0; i < 10000000; i++ {
		c, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
		}

		c.Write(handshakePayload)
		c.Write(loginStartPayload)
	}
}
