package infrared

import (
	"fmt"
	"log"
	"strings"

	"github.com/haveachin/infrared/net"
	"github.com/haveachin/infrared/net/packet"
)

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	// TCP address to listen on
	Addr string

	// Map of domains that to configs
	Configs map[string]Config
}

// ListenAndServe listens on the TCP network address addr and then handle packets
// on incoming connections.
func (s *Proxy) ListenAndServe() error {
	log.Println("Starting to Listen on", s.Addr)
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

func makeHandshake(client *net.Conn, config Config) error {
	pk, err := client.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != packet.SLPRequestPacketID {
		return fmt.Errorf("expexted request packet (%d); got this: %d", packet.SLPRequestPacketID, pk.ID)
	}

	response := packet.SLPResponse{
		JSONResponse: packet.String(config.PlaceholderJSON),
	}

	if err := client.WritePacket(response.Marshal()); err != nil {
		return err
	}

	pk, err = client.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != packet.SLPPingPacketID {
		return fmt.Errorf("expexted ping packet (%d); got this: %d", packet.SLPPingPacketID, pk.ID)
	}

	return client.WritePacket(pk)
}

func sendDisconnectMessage(client *net.Conn, message, address string) error {
	pk, err := client.ReadPacket()
	if err != nil {
		return err
	}

	start, err := packet.ParseLoginStart(pk)
	if err != nil {
		return err
	}

	log.Printf("%s[%s] tried to connect over %s", start.Name, client.Socket.RemoteAddr().String(), address)

	message = strings.Replace(message, "$username", string(start.Name), -1)

	disconnect := packet.LoginDisconnect{
		Reason: packet.Chat(fmt.Sprintf("{\"text\":\"%s\"}", message)),
	}

	return client.WritePacket(disconnect.Marshal())
}

func (s *Proxy) handleConn(conn *net.Conn) {
	pk, err := conn.ReadPacket()
	if err != nil {
		log.Println(err)
		return
	}

	handshake, err := packet.ParseSLPHandshake(pk)
	if err != nil {
		log.Println(err)
		return
	}

	config := s.Configs[string(handshake.ServerAddress)]

	rconn, err := net.DialMC(config.ProxyTo)
	if err != nil {
		if handshake.RequestsStatus() {
			if err := makeHandshake(conn, config); err != nil {
				log.Println("Handshake failed:", err)
			}

			return
		}

		address := fmt.Sprintf("%s:%d", handshake.ServerAddress, handshake.ServerPort)

		if err := sendDisconnectMessage(conn, config.DisconnectMessage, address); err != nil {
			log.Println("Disconnect failed:", err)
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
