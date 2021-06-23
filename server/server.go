package server

import (
	"errors"
	"fmt"
	"io"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
)

var (
	ErrCantConnectWithServer = errors.New("cant connect with server")
)

type ServerConfig struct {
	NumberOfInstances int `json:"numberOfInstances"`

	DomainName string   `json:"domainName"`
	SubDomains []string `json:"subDomains"`

	ListenTo 		  string `json:"listenTo"`
	ProxyBind         string `json:"proxyBind"`
	SendProxyProtocol bool   `json:"sendProxyProtocol"`
	ProxyTo           string `json:"proxyTo"`
	RealIP            bool   `json:"realIp"`

	DialTimeout       int    `json:"dialTimeout"`
	DisconnectMessage string `json:"disconnectMessage"`

	//Need different statusconfig struct
	OnlineStatus  infrared.StatusConfig `json:"onlineStatus"`
	OfflineStatus infrared.StatusConfig `json:"offlineStatus"`
}

func NewMCServer(connFactory connection.ServerConnFactory, connCh <-chan connection.HandshakeConn, closeCh <-chan struct{}) MCServer {
	return MCServer{
		ConnFactory: connFactory,
		ConnCh:      connCh,
		CloseCh:     closeCh,
	}
}

type MCServer struct {
	Config              ServerConfig
	ConnFactory         connection.ServerConnFactory
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet

	ConnCh  <-chan connection.HandshakeConn
	CloseCh <-chan struct{}

	JoiningActions []func(domain string)
	LeavingActions []func(domain string)
}

func (s *MCServer) Status(conn connection.HandshakeConn) protocol.Packet {
	serverConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs := conn.HandshakePacket
	serverConn.WritePacket(hs)
	serverConn.WritePacket(protocol.Packet{})
	pk, err := serverConn.ReadPacket()
	if err == nil {
		if len(s.OnlineConfigStatus.Data) != 0 {
			pk = s.OnlineConfigStatus
		}
	} else if len(s.OfflineConfigStatus.Data) != 0 {
		pk = s.OfflineConfigStatus
	} else {
		pk, _ = infrared.StatusConfig{}.StatusResponsePacket()
	}
	return pk
}

// Error testing needed
func (s *MCServer) Login(conn connection.HandshakeConn) error {
	serverConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs := conn.HandshakePacket
	serverConn.WritePacket(hs)
	pk, _ := conn.ReadPacket()
	serverConn.WritePacket(pk)

	go func(client, server connection.PipeConn) {
		for _, action := range s.JoiningActions {
			action(s.Config.DomainName)
		}

		clientConn := client.Conn()
		serverConn := server.Conn()
		go func() {
			io.Copy(serverConn, clientConn)
			clientConn.Close()
		}()
		io.Copy(clientConn, serverConn)
		serverConn.Close()

		for _, action := range s.LeavingActions {
			action(s.Config.DomainName)
		}
	}(conn, serverConn)

	return nil
}

func (s *MCServer) Start() {
ForLoop:
	for {
		select {
		case conn := <-s.ConnCh:
			switch connection.ParseRequestType(conn) {
			case connection.LoginRequest:
				s.Login(conn)
			case connection.StatusRequest:
				err := s.handleStatusRequest(conn)
				if err != nil {
					fmt.Println(err)
				}
			default:
				fmt.Sprintln("Didnt recognize handshake id")
			}
		case <-s.CloseCh:
			break ForLoop
		}

	}
}

func (s *MCServer) handleStatusRequest(conn connection.HandshakeConn) error {
	// Read the request packet and send status response back
	_, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	responsePk := s.Status(conn)

	if err := conn.WritePacket(responsePk); err != nil {
		return err
	}
	// This ping packet is optional, clients send them but scripts like bots dont have to send them
	//  and this will return an EOF when the connections gets closed
	pingPk, err := conn.ReadPacket()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		fmt.Println(err)
		return err
	}
	return conn.WritePacket(pingPk)
}
