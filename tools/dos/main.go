package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"log"
	"net"
	"os"
	"runtime"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/pkg/infrared/protocol/login"
	"github.com/pires/go-proxyproto"
)

var (
	handshakePayload  []byte
	loginStartPayload []byte
)

func initPayload() {
	var pk protocol.Packet
	_ = handshaking.ServerBoundHandshake{
		ProtocolVersion: 758,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       handshaking.StateStatusServerBoundHandshake,
	}.Marshal(&pk)
	var buf bytes.Buffer
	_, _ = pk.WriteTo(&buf)
	handshakePayload = buf.Bytes()

	_ = login.ServerBoundLoginStart{
		Name:          "Test",
		HasSignature:  false,
		HasPlayerUUID: false,
	}.Marshal(&pk, protocol.Version1_19)
	buf.Reset()
	_, _ = pk.WriteTo(&buf)
	loginStartPayload = buf.Bytes()

	log.Println(len(handshakePayload) + len(loginStartPayload))
}

var (
	sendProxyProtocol = false
)

func initFlags() {
	flag.BoolVar(&sendProxyProtocol, "p", sendProxyProtocol, "sends a proxy protocol v2 header before its payload")
	flag.Parse()
}

func main() {
	runtime.GOMAXPROCS(4)

	initFlags()
	initPayload()

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

	for i := 0; ; i++ {
		if i > 0 && i%10 == 0 {
			log.Printf("%d requests sent\n", i)
		}

		go func() {
			c, err := net.Dial("tcp", targetAddr)
			if err != nil {
				return
			}

			if sendProxyProtocol {
				if err = writeProxyProtocolHeader(randomAddr(), c); err != nil {
					log.Printf("Write proxy protocol: %s", err)
				}
			}

			_, _ = c.Write(handshakePayload)
			_, _ = c.Write(loginStartPayload)
			_ = c.Close()
		}()
		// time.Sleep(time.Millisecond * 10)
	}
}

func randomAddr() net.Addr {
	addrBytes := make([]byte, 6)
	_, _ = rand.Read(addrBytes)

	return &net.TCPAddr{
		IP:   net.IPv4(addrBytes[0], addrBytes[1], addrBytes[2], addrBytes[3]),
		Port: int(addrBytes[4])*256 + int(addrBytes[5]),
	}
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	tp := proxyproto.TCPv4
	addrTCP, ok := addr.(*net.TCPAddr)
	if !ok {
		panic("not a tcp connection")
	}

	if addrTCP.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        addr,
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}
