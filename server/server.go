package server

import (
	"errors"
	"io"
	"log"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/pires/go-proxyproto"
)

var (
	ErrCantWriteToServer     = errors.New("can't write to proxy target")
	ErrCantWriteToClient     = errors.New("can't write to client")
	ErrCantConnectWithServer = errors.New("cant connect with server")
	ErrInvalidHandshakeID    = errors.New("didnt recognize handshake id")
)

type MCServer struct {
	CreateServerConn  connection.ServerConnFactory
	SendProxyProtocol bool
	RealIP            bool

	OnlineConfigStatus  protocol.Packet
	OfflineConfigStatus protocol.Packet

	ConnCh         <-chan connection.HandshakeConn
	CloseCh        <-chan struct{}
	JoiningActions []func()
	LeavingActions []func()
}

func (s *MCServer) Status(conn connection.HandshakeConn) protocol.Packet {
	var pk protocol.Packet
	var isServerOnline bool
	serverConn, err := s.CreateServerConn()

	if err == nil {
		if err := serverConn.WritePacket(conn.HandshakePacket); err == nil {
			if err := serverConn.WritePacket(protocol.Packet{}); err == nil {
				pk, err = serverConn.ReadPacket()
				isServerOnline = true
			}
		}
	}

	if isServerOnline {
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

func (s *MCServer) Login(conn connection.HandshakeConn) error {
	serverConn, err := s.CreateServerConn()
	if err != nil {
		return err
	}

	if s.SendProxyProtocol {
		header := &proxyproto.Header{
			Version:           2,
			Command:           proxyproto.PROXY,
			TransportProtocol: proxyproto.TCPv4,
			SourceAddr:        conn.RemoteAddr(),
			DestinationAddr:   serverConn.Conn().RemoteAddr(),
		}

		if _, err = header.WriteTo(serverConn.Conn()); err != nil {
			return ErrCantWriteToServer
		}
	}

	var handshakePk protocol.Packet
	if s.RealIP {
		hs := conn.Handshake
		hs.UpgradeToRealIP(conn.RemoteAddr(), time.Now())
		handshakePk = hs.Marshal()
	} else {
		handshakePk = conn.HandshakePacket
	}

	if err := serverConn.WritePacket(handshakePk); err != nil {
		return err
	}
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}
	if err := serverConn.WritePacket(pk); err != nil {
		return err
	}

	go func(client, server connection.PipeConn, joinActions, leaveActions []func()) {
		for _, action := range s.JoiningActions {
			action()
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
			action()
		}
	}(conn, serverConn, s.JoiningActions, s.LeavingActions)

	return nil
}

func (s *MCServer) Start() {
ForLoop:
	for {
		select {
		case conn := <-s.ConnCh:
			go func() {
				var err error
				switch connection.ParseRequestType(conn) {
				case connection.LoginRequest:
					err = s.Login(conn)
				case connection.StatusRequest:
					err = s.handleStatusRequest(conn)
				default:
					err = ErrInvalidHandshakeID
				}
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					log.Printf("error: %v", err)
					conn.Conn().Close()
				}
			}()
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
	pingPk, err := conn.ReadPacket()
	if err != nil {
		return err
	}
	return conn.WritePacket(pingPk)
}
