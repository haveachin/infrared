package infrared

import (
	"log"

	"github.com/haveachin/infrared/net"
	"github.com/haveachin/infrared/net/packet"
)

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	// TCP address to listen on
	Addr string

	// Map of domains that routes to server addresses
	Routes map[string]string

	// Map of domains that routes to place holder packets
	PlaceHolder map[string][]byte
}

// ListenAndServe listens on the TCP network address addr and then handle packets
// on incoming connections.
func (s *Proxy) ListenAndServe() error {
	listener, err := net.ListenMC(s.Addr)
	if err != nil {
		return err
	}

	return s.serve(listener)
}

func (s *Proxy) serve(listener *net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go s.handleConn(&conn)
	}
}

func (s *Proxy) handleConn(conn *net.Conn) {
	pk, _ := conn.ReadPacket()
	handshake, _ := packet.ParseHandshake(pk)
	rconn, err := net.DialMC(s.Routes[string(handshake.ServerAddress)])
	if err != nil {
		err := conn.WritePacket(packet.Marshal(0x00, packet.String(`{
    "version": {
        "name": "1.14.4",
        "protocol": 498
    },
    "players": {
        "max": 0,
        "online": 1,
        "sample": [
            {
                "name": "thinkofdeath",
                "id": "4566e69f-c907-48ee-8d71-d7ba5aa00d20"
            }
        ]
    },	
    "description": {
        "text": "Hello world"
    },
    "favicon": "data:image/png;base64,<data>"
}`)))
		if err != nil {
			log.Println(err)
		}

		return
	}

	var pipe = func(src, dst *net.Conn) {
		defer func() {
			_ = conn.Close()
			_ = rconn.Close()
		}()

		buffer := make([]byte, 65535)

		for {
			n, err := src.Socket.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			_, err = dst.Socket.Write(data)
			if err != nil {
				return
			}
		}
	}

	go pipe(conn, rconn)
	go pipe(rconn, conn)

	if err := rconn.WritePacket(pk); err != nil {
		log.Println(err)
	}
}
