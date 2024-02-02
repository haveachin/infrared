package infrared

import (
	"bufio"
	"net"
	"testing"

	"github.com/pires/go-proxyproto"
)

type TestConn struct {
	net.Conn
}

func (c *TestConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 25565,
	}
}

func TestInfrared_handlePipe_ProxyProtocol(t *testing.T) {
	rcIn, rcOut := net.Pipe()
	_, cOut := net.Pipe()

	c := TestConn{Conn: cOut}
	rc := TestConn{Conn: rcIn}

	srv := New()

	go func() {
		resp := ServerResponse{
			ServerConn:        newConn(&rc),
			SendProxyProtocol: true,
		}

		_ = srv.handlePipe(newConn(&c), resp)
	}()

	r := bufio.NewReader(rcOut)
	header, err := proxyproto.Read(r)
	if err != nil {
		t.Fatalf("Unexpected error reading proxy protocol header: %v", err)
	}

	if header.Command != proxyproto.PROXY {
		t.Fatalf("Unexpected proxy protocol command: %v", header.Command)
	}

	if header.TransportProtocol != proxyproto.TCPv4 {
		t.Fatalf("Unexpected proxy protocol transport protocol: %v", header.TransportProtocol)
	}

	if header.Version != 2 {
		t.Fatalf("Unexpected proxy protocol version: %v", header.Version)
	}
}

func TestInfrared_handlePipe_NoProxyProtocol(t *testing.T) {
	rcIn, rcOut := net.Pipe()
	_, cOut := net.Pipe()

	c := TestConn{Conn: cOut}
	rc := TestConn{Conn: rcIn}

	srv := New()

	go func() {
		resp := ServerResponse{
			ServerConn:        newConn(&rc),
			SendProxyProtocol: false,
		}

		_ = srv.handlePipe(newConn(&c), resp)
	}()

	r := bufio.NewReader(rcOut)
	if _, err := proxyproto.Read(r); err == nil {
		t.Fatal("Expected error reading proxy protocol header, but got nothing")
	}
}
