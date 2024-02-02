package infrared

import (
	"bufio"
	"net"
	"testing"

	"github.com/pires/go-proxyproto"
)

type ProxyProtocolTesterConn struct {
	net.Conn
}

func (c *ProxyProtocolTesterConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 25565,
	}
}

func TestProxyProtocolhandlePipe(t *testing.T) {
	serverConnIn, serverConnOut := net.Pipe()
	_, clientConnOut := net.Pipe()

	clientConn := ProxyProtocolTesterConn{Conn: clientConnOut}

	ir := New()

	testConn := ProxyProtocolTesterConn{Conn: serverConnIn}

	reqResponse := ServerResponse{
		ServerConn:        newConn(&testConn),
		SendProxyProtocol: true,
	}

	go func() {
		_ = ir.handlePipe(newConn(&clientConn), reqResponse)
	}()

	bufReader := bufio.NewReader(serverConnOut)
	header, err := proxyproto.Read(bufReader)

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

func TestNoProxyProtocolhandlePipe(t *testing.T) {
	serverConnIn, serverConnOut := net.Pipe()
	_, clientConnOut := net.Pipe()

	clientConn := ProxyProtocolTesterConn{Conn: clientConnOut}

	ir := New()

	testConn := ProxyProtocolTesterConn{Conn: serverConnIn}

	reqResponse := ServerResponse{
		ServerConn:        newConn(&testConn),
		SendProxyProtocol: false,
	}

	go func() {
		_ = ir.handlePipe(newConn(&clientConn), reqResponse)
	}()

	bufReader := bufio.NewReader(serverConnOut)
	_, err := proxyproto.Read(bufReader)

	if err == nil {
		t.Fatal("Expected error reading proxy protocol header, but got nothing")
	}
}
