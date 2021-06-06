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

type LoginServer interface {
	Login(conn connection.LoginConn) error
}

type StatusServer interface {
	Status(conn connection.StatusConn) protocol.Packet
}

// Server will act as abstraction layer between connection and proxy ...?
type Server interface {
	LoginServer
	StatusServer
	Start()
}

type MCServer struct {
	Config              ServerConfig
	ConnFactory         connection.ServerConnFactory
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet

	ConnCh <-chan connection.HandshakeConn
}

func (s *MCServer) Status(clientConn connection.StatusConn) protocol.Packet {
	serverConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs := clientConn.HandshakePacket()
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
func (s *MCServer) Login(conn connection.LoginConn) error {
	sConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs := conn.HandshakePacket()
	sConn.WritePacket(hs)
	pk, _ := conn.ReadPacket()
	sConn.WritePacket(pk)

	// Doing it like this should prevent weird behavior with can be causes by closujers combined with goroutines
	go func(client, server connection.PipeConn) {
		// closing connections on disconnect happen in the Pipe function
		connection.Pipe(client, server)
	}(conn, sConn)

	return nil
}

func (s *MCServer) Start() {
	for {
		c := <-s.ConnCh
		conn := c.(connection.HandshakeConn)
		switch connection.ParseRequestType(conn) {
		case connection.LoginRequest:
			lConn := conn.(connection.LoginConn)
			s.Login(lConn)
		case connection.StatusRequest:
			sConn := conn.(connection.StatusConn)
			err := s.handleStatusRequest(sConn)
			if err != nil {
				fmt.Println(err)
			}
		default:
			fmt.Sprintln("Didnt recognize handshake id")
		}
	}
}

func (s *MCServer) handleStatusRequest(conn connection.StatusConn) error {
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
