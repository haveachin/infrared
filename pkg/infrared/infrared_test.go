package infrared

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
	"github.com/pires/go-proxyproto"
)

type mockServerRequestResponder struct{}

func (r mockServerRequestResponder) RespondeToServerRequest(req ServerRequest, srv *Server) {
	req.ResponseChan <- ServerRequestResponse{
		StatusResponse: protocol.Packet{
			ID:   0x1337,
			Data: []byte{0x13, 0x37},
		},
	}
}

func BenchmarkInfrared_handleConn(b *testing.B) {
	var hsStatusPk protocol.Packet
	handshaking.ServerBoundHandshake{
		ProtocolVersion: 1337,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       handshaking.StateStatusServerBoundHandshake,
	}.Marshal(&hsStatusPk)
	var statusPk protocol.Packet
	status.ServerBoundRequest{}.Marshal(&statusPk)
	var pingPk protocol.Packet
	pingPk.Encode(0x01)

	tt := []struct {
		name string
		pks  []protocol.Packet
	}{
		{
			name: "status_handshake",
			pks: []protocol.Packet{
				hsStatusPk,
				statusPk,
				pingPk,
			},
		},
	}

	for _, tc := range tt {
		in, out := net.Pipe()
		sgInChan := make(chan ServerRequest)
		srv, err := NewServer(func(cfg *ServerConfig) {
			*cfg = ServerConfig{
				Domains: []ServerDomain{
					"localhost",
				},
				Addresses: []ServerAddress{
					"localhost:25566",
				},
			}
		})
		if err != nil {
			b.Error(err)
			return
		}

		sg := serverGateway{
			servers: []*Server{
				srv,
			},
			responder: mockServerRequestResponder{},
		}
		go func() {
			if err := sg.listenAndServe(sgInChan); err != nil {
				b.Error(err)
			}
		}()
		c := newConn(out)
		c.srvReqChan = sgInChan

		var buf bytes.Buffer
		for _, pk := range tc.pks {
			if _, err := pk.WriteTo(&buf); err != nil {
				b.Error(err)
			}
		}

		ir := New()
		if err := ir.init(); err != nil {
			b.Error(err)
		}

		go func() {
			b := make([]byte, 0xffff)
			for {
				_, err := in.Read(b)
				if err != nil {
					return
				}
			}
		}()

		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				go in.Write(buf.Bytes())
				if err := ir.handleConn(c); err != nil && !errors.Is(err, io.EOF) {
					b.Error(err)
				}
			}
		})

		in.Close()
		out.Close()
	}
}

type MockListener struct {
	in <-chan net.Conn
}

func (l *MockListener) Accept() (net.Conn, error) {
	return <-l.in, nil
}

func (l *MockListener) Close() error {
	return nil
}

func (l *MockListener) Addr() net.Addr {
	return nil
}

func BenchmarkInfrared_ListenAndServe(b *testing.B) {
	var hsStatusPk protocol.Packet
	handshaking.ServerBoundHandshake{
		ProtocolVersion: 1337,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       handshaking.StateStatusServerBoundHandshake,
	}.Marshal(&hsStatusPk)
	var statusPk protocol.Packet
	status.ServerBoundRequest{}.Marshal(&statusPk)
	var pingPk protocol.Packet
	pingPk.Encode(0x01)

	connInChan := make(chan net.Conn)
	ir := Infrared{
		l: &MockListener{
			in: connInChan,
		},
		sg: serverGateway{
			servers: []*Server{
				{
					cfg: ServerConfig{
						Domains: []ServerDomain{
							"localhost",
						},
						Addresses: []ServerAddress{
							"localhost:25566",
						},
					},
				},
			},
			responder: mockServerRequestResponder{},
		},
		bufPool: sync.Pool{
			New: func() any {
				b := make([]byte, 1<<15)
				return &b
			},
		},
		conns: make(map[net.Addr]*conn),
	}

	sgInChan := make(chan ServerRequest)
	go ir.sg.listenAndServe(sgInChan)
	go ir.listenAndServe(sgInChan)

	for i := 0; i < b.N; i++ {
		in, out := net.Pipe()
		outConn := newConn(out)
		outConn.WritePackets(hsStatusPk, statusPk)

		connInChan <- in

		outConn.ReadPackets(&protocol.Packet{})
		outConn.WritePackets(pingPk)
		outConn.ReadPackets(&protocol.Packet{})
		in.Close()
		out.Close()
	}
}

type ProxyProtocolTesterConn struct {
	net.Conn
	c net.Conn
}

func (c *ProxyProtocolTesterConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 25565}
}

func (c *ProxyProtocolTesterConn) Read(b []byte) (int, error) {
	return c.c.Read(b)
}

func (c *ProxyProtocolTesterConn) Write(b []byte) (int, error) {
	return c.c.Write(b)
}

func (c *ProxyProtocolTesterConn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

func (c *ProxyProtocolTesterConn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func TestProxyProtocolhandlePipe(t *testing.T) {
	serverConnIn, serverConnOut := net.Pipe()
	_, clientConnOut := net.Pipe()

	clientConn := ProxyProtocolTesterConn{c: clientConnOut}

	ir := New()

	testConn := ProxyProtocolTesterConn{c: serverConnIn}

	reqResponse := ServerRequestResponse{
		ServerConn:        newConn(&testConn),
		SendProxyProtocol: true,
	}

	go ir.handlePipe(newConn(&clientConn), reqResponse)

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

	clientConn := ProxyProtocolTesterConn{c: clientConnOut}

	ir := New()

	testConn := ProxyProtocolTesterConn{c: serverConnIn}

	reqResponse := ServerRequestResponse{
		ServerConn:        newConn(&testConn),
		SendProxyProtocol: false,
	}

	go ir.handlePipe(newConn(&clientConn), reqResponse)

	bufReader := bufio.NewReader(serverConnOut)
	_, err := proxyproto.Read(bufReader)

	if err == nil {
		t.Fatal("Expected error reading proxy protocol header, but got nothing")
	}

}
