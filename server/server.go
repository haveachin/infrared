package server

import (
	"errors"
	"fmt"

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
	ConnFactory         func() connection.ServerConnection //Probably needs better names, or a different code structure
	serverConn          connection.ServerConnection
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet
	UseConfigStatus     bool

	ConnCh <-chan connection.HSConnection
}

func (s *MCServer) Status(conn connection.StatusConnection) protocol.Packet {
	if s.serverConn == nil {
		s.serverConn = s.ConnFactory()
	}
	pk, err := s.serverConn.Status()
	if err == nil {
		if s.UseConfigStatus {
			pk = s.OnlineConfigStatus
		}
	} else if s.UseConfigStatus {
		pk = s.OfflineConfigStatus
	} else {
		pk, _ = infrared.StatusConfig{}.StatusResponsePacket()
	}
	return pk
}

func (s *MCServer) Login(conn connection.LoginConnection) error {
	sConn := s.ConnFactory()
	hs, _ := conn.HsPk()
	sConn.SendPK(hs)
	pk, _ := conn.LoginStart()
	if err := sConn.SendPK(pk); err != nil {
		return err
	}

	go func() {
		connection.Pipe(conn, sConn)
	}()

	return nil
}

func (s *MCServer) Start() {
	for {
		conn := <-s.ConnCh
		switch connection.ParseRequestType(conn) {
		case connection.LoginRequest:
			lConn := conn.(connection.LoginConnection)
			s.Login(lConn)
		case connection.StatusRequest:
			sConn := conn.(connection.StatusConnection)
			status := s.Status(sConn)
			conn.WritePacket(status)
		default:
			fmt.Sprintln("Didnt recognize handshake id")
		}
	}
}

