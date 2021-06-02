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

type LoginServer interface {
	Login(conn connection.LoginConnection) error
}

type StatusServer interface {
	Status(conn connection.StatusConnection) protocol.Packet
}

// Server will act as abstraction layer between connection and proxy ...?
type Server interface {
	LoginServer
	StatusServer
}

type MCServer struct {
	Config              infrared.ProxyConfig
	ConnFactory         connection.ServerConnFactory
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet

	ConnCh <-chan connection.GatewayConnection
}

func (s *MCServer) Status(clientConn connection.StatusConnection) protocol.Packet {
	serverConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs, _ := clientConn.HsPk()
	serverConn.SendPK(hs)
	pk, err := serverConn.Status(protocol.Packet{})
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
func (s *MCServer) Login(conn connection.LoginConnection) error {
	sConn, _ := s.ConnFactory(s.Config.ProxyTo)
	hs, _ := conn.HsPk()
	sConn.SendPK(hs)
	pk, _ := conn.LoginStart()
	sConn.SendPK(pk)
	go func() {
		// Disconnection might need some more attention
		connection.Pipe(conn, sConn)
	}()

	return nil
}

func (s *MCServer) Start() {
	for {
		c := <-s.ConnCh
		conn := c.(connection.HSConnection)
		switch connection.ParseRequestType(conn) {
		case connection.LoginRequest:
			lConn := conn.(connection.LoginConnection)
			s.Login(lConn)
		case connection.StatusRequest:
			sConn := conn.(connection.StatusConnection)
			err := s.handleStatusRequest(sConn)
			if err != nil {
				fmt.Println(err)
			}
		default:
			fmt.Sprintln("Didnt recognize handshake id")
		}
	}
}

func (s *MCServer) handleStatusRequest(conn connection.StatusConnection) error {
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
		if err == io.EOF {
			return nil
		}
		return err
	}

	return conn.WritePacket(pingPk)
}
