package main

import (
	"crypto/rand"
	"log"
	"net"
	"time"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/pires/go-proxyproto"
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
		Name:          "Test",
		HasPublicKey:  false,
		HasPlayerUUID: false,
	}
	pk = loginStart.Marshal(protocol.Version_1_19)
	bb, err = pk.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	loginStartPayload = bb
}

func main() {
	for i := 0; ; i++ {
		if i > 0 && i%1000 == 0 {
			log.Printf("%d requests sent\n", i)
		}

		c, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
		}

		//writeProxyProtocolHeader(randomAddr(), c)
		c.Write(handshakePayload)
		c.Write(loginStartPayload)
		time.Sleep(time.Millisecond)
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
