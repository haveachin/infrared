package server

import (
	"errors"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
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

var (
	ErrCantConnectWithServer = errors.New("cant connect with server")
)

type MCServer struct {
	ConnFactory         func() connection.ServerConnection //Probably needs better names, or a different code structure
	serverConn          connection.ServerConnection
	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet
	UseConfigStatus     bool
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

	connection.Pipe(conn, sConn)

	return nil
}

func HandleStatusRequest(conn connection.StatusConnection, server Server) error {
	status := server.Status(conn)
	return conn.WritePacket(status)
}

func HandleLoginRequest(conn connection.LoginConnection, server Server) error {
	return server.Login(conn)
}
