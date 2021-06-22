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
	NumberOfInstances int    `json:"numberOfInstances"`
	DomainName        string `json:"domainName"`
	ProxyTo           string `json:"proxyTo"`
	RealIP            bool   `json:"realIp"`

	//Need different statusconfig struct
	OnlineStatus infrared.StatusConfig `json:"onlineStatus"`
	//Need different statusconfig struct
	OfflineStatus infrared.StatusConfig `json:"offlineStatus"`
}

type MCServer struct {
	Config              ServerConfig
	ConnFactory         connection.ServerConnFactory
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet

	ConnCh <-chan connection.HandshakeConn

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
	for {
		conn := <-s.ConnCh
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
