package infrared

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

type mockServerRequestResponder struct {
}

func (r mockServerRequestResponder) RespondeToServerRequest(req ServerRequest, srv *Server) {
	req.ResponseChan <- ServerRequestResponse{}
}

func BenchmarkHandleConn_Status(b *testing.B) {
	var hsPk protocol.Packet
	handshaking.ServerBoundHandshake{
		ProtocolVersion: 1337,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       handshaking.StateStatusServerBoundHandshake,
	}.Marshal(&hsPk)
	var statusPk protocol.Packet
	status.ServerBoundRequest{}.Marshal(&statusPk)
	var pingPk protocol.Packet
	pingPk.Encode(0x01)

	tt := []struct {
		name string
		pks  []protocol.Packet
	}{
		{
			name: "normal_handshake",
			pks: []protocol.Packet{
				hsPk,
				statusPk,
				pingPk,
			},
		},
	}

	for _, tc := range tt {
		sgInChan := make(chan ServerRequest)
		sg := serverGateway{
			Servers: []*Server{
				NewServer(func(cfg *ServerConfig) {
					*cfg = ServerConfig{
						Domains: []ServerDomain{
							"localhost",
						},
					}
				}),
			},
			requestChan: sgInChan,
			responder:   mockServerRequestResponder{},
		}
		go func() {
			if err := sg.listenAndServe(); err != nil {
				b.Error(err)
			}
		}()
		in, out := net.Pipe()
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
