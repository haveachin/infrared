package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/pires/go-proxyproto"
)

const (
	bind                   = ":25511"
	target                 = ""
	receiveProxyProtocolV2 = true
	sendProxyProtocolV2    = true
)

func main() {
	l, err := net.Listen("tcp", bind)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}

		if receiveProxyProtocolV2 {
			_, err := proxyproto.Read(bufio.NewReader(c))
			if err != nil {
				log.Printf("proxyproto.Read: %v", err)
				continue
			}
		}

		rc, err := net.Dial("tcp", target)
		if err != nil {
			log.Printf("dial: %v", err)
			c.Close()
			continue
		}

		if sendProxyProtocolV2 {
			if err := writeProxyProtocolHeader(c.RemoteAddr(), rc); err != nil {
				log.Printf("writeProxyProtocolHeader: %v", err)
				c.Close()
				rc.Close()
				continue
			}
		}

		go func() {
			if err := openCopy(rc, c, "C->S"); err != nil {
				log.Println(err)
			}
			rc.Close()
			c.Close()
		}()

		if err := openCopy(c, rc, "S->C"); err != nil {
			log.Println(err)
		}

		rc.Close()
		c.Close()
	}
}

func openCopy(dst io.Writer, src io.Reader, logPrefix string) error {
	buf := make([]byte, 0xffff)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return fmt.Errorf("%s: src closed: %w", logPrefix, err)
		}

		data := buf[:n]
		log.Printf("%s: %v", logPrefix, data)

		if _, err := dst.Write(data); err != nil {
			return fmt.Errorf("%s: dst closed: %w", logPrefix, err)
		}
	}
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	tp := proxyproto.TCPv4
	tcpAddr := addr.(*net.TCPAddr)
	if tcpAddr.IP.To4() == nil {
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
