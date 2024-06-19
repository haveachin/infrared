package main

import (
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
)

var statusRequestPayload = []byte{0x00}

func handshake(w io.Writer, protVer int32) {
	var pk protocol.Packet
	handshaking.ServerBoundHandshake{
		ProtocolVersion: protocol.VarInt(protVer),
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       handshaking.StateStatusServerBoundHandshake,
	}.Marshal(&pk)
	pk.WriteTo(w)
}

func statusRequest(w io.Writer) {
	var pk protocol.Packet
	pk.Encode(0x00)
	pk.WriteTo(w)
}

func main() {
	runtime.GOMAXPROCS(4)

	targetAddr := "localhost:25565"

	if len(os.Args) < 2 {
		log.Println("No target address specified")
		log.Printf("Defaulting to %s\n", targetAddr)
	} else {
		targetAddr = os.Args[1]
	}

	conn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Fatal(err)
	}
	_ = conn.Close()

	wg := sync.WaitGroup{}

	for i := 0; i <= 766; i++ {
		log.Printf("Protocol version %d requests sent\n", i)

		wg.Add(1)
		go func(protVer int32) {
			defer func() {
				log.Printf("Done %d\n", protVer)
				wg.Done()
			}()

			c, err := net.Dial("tcp", targetAddr)
			if err != nil {
				return
			}

			handshake(c, protVer)
			statusRequest(c)
			var pk protocol.Packet
			_, _ = pk.ReadFrom(c)
			_ = c.Close()
		}(int32(i))
	}

	wg.Wait()
}
