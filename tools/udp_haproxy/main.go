package main

import (
	"log"
	"net"
	"sync"

	"github.com/pires/go-proxyproto"
)

const (
	listenTo = ":19134"
	proxyTo  = ":19132"
)

var conns sync.Map

func main() {
	conn, err := net.ListenPacket("udp", listenTo)
	if err != nil {
		log.Fatal(err)
	}

	b := make([]byte, 1500)
	for {
		n, addr, err := conn.ReadFrom(b)
		if err != nil {
			log.Fatal(err)
		}

		value, found := conns.Load(addr.String())
		if !found {
			log.Println("new conn")
			rconn, err := net.Dial("udp", proxyTo)
			if err != nil {
				log.Fatal(err)
			}
			/*if err := writeProxyProtocolHeader(addr, rconn); err != nil {
				log.Println(err)
				continue
			}*/
			conns.Store(addr.String(), rconn)
			if _, err := rconn.Write(b[:n]); err != nil {
				log.Println(err)
			}
			go work(conn, rconn, addr)
			continue
		}

		rconn := value.(net.Conn)
		if _, err := rconn.Write(b[:n]); err != nil {
			log.Println(err)
		}
	}
}

func work(conn net.PacketConn, rconn net.Conn, addr net.Addr) {
	b := make([]byte, 1500)
	for {
		n, err := rconn.Read(b)
		if err != nil {
			log.Println(err)
			return
		}

		_, err = conn.WriteTo(b[:n], addr)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func writeProxyProtocolHeader(connAddr net.Addr, rc net.Conn) error {
	tp := proxyproto.UDPv4
	addr := connAddr.(*net.UDPAddr)
	if addr.IP.To4() == nil {
		tp = proxyproto.UDPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        connAddr,
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}

	return nil
}
