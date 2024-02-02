package main

import (
	"bytes"
	"log"
	"net"
	"sync"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
)

var payload []byte

func initPayload() {
	buf := new(bytes.Buffer)
	protocol.VarInt(0x200000).WriteTo(buf)
	protocol.VarInt(handshaking.ServerBoundHandshakeID).WriteTo(buf)
	protocol.VarInt(protocol.Version1_20_2.ProtocolNumber()).WriteTo(buf)
	protocol.String("localhost").WriteTo(buf)
	protocol.UnsignedShort(25565).WriteTo(buf)
	protocol.Byte(2).WriteTo(buf)
	payload = buf.Bytes()
}

func main() {
	initPayload()

	n := 100000

	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			c, err := net.Dial("tcp", "localhost:25565")
			if err != nil {
				log.Println(err)
				return
			}

			if _, err = c.Write(payload); err != nil {
				log.Println(err)
			}
			_ = c.Close()
			wg.Done()
		}()
	}

	wg.Wait()
}
