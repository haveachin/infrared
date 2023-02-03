package main

import (
	"crypto/rand"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/pires/go-proxyproto"
)

var (
	handshakePayload  []byte
	loginStartPayload []byte
)

func initPayload() {
	handshake := handshaking.ServerBoundHandshake{
		ProtocolVersion: 758,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       2,
	}
	pk := handshake.Marshal()
	handshakePayload = pk.Marshal()

	loginStart := login.ServerLoginStart{
		Name:          "Test",
		HasPublicKey:  false,
		HasPlayerUUID: false,
	}
	pk = loginStart.Marshal(protocol.Version_1_19)
	loginStartPayload = pk.Marshal()

	log.Println(len(handshakePayload) + len(loginStartPayload))
}

var (
	sendProxyProtocol = false
)

func initFlags() {
	flag.BoolVar(&sendProxyProtocol, "p", sendProxyProtocol, "sends a proxy protocol v2 header before its payload")
	flag.Parse()
}

func init() {
	initFlags()
	initPayload()
}

func main() {
	targetAddr := "localhost:25565"

	if len(os.Args) < 2 {
		log.Println("No target address specified")
		log.Printf("Defaulting to %s\n", targetAddr)
	} else {
		targetAddr = os.Args[1]
	}

	c, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Fatal(err)
	}
	c.Close()

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
				writeProxyProtocolHeader(randomAddr(), c)
			}

			c.Write(handshakePayload)
			c.Write(loginStartPayload)
			go io.ReadAll(c)
			c.Write([]byte("dfighusdlgh"))
		}()
		//time.Sleep(time.Millisecond * 10)
	}
}

func randomAddr() net.Addr {
	addrBytes := make([]byte, 6)
	rand.Read(addrBytes)

	return &net.TCPAddr{
		IP:   net.IPv4(addrBytes[0], addrBytes[1], addrBytes[2], addrBytes[3]),
		Port: int(addrBytes[4])*256 + int(addrBytes[5]),
	}
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	tp := proxyproto.TCPv4
	addrTCP := addr.(*net.TCPAddr)
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
