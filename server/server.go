package server

import (
	"errors"
	"io"
	"log"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
)

var (
	ErrCantConnectWithServer = errors.New("cant connect with server")
)

type ServerConfig struct {
	// MainDomain will be treated as primary key (refactor: imperative -> workerpools)
	MainDomain   string   `json:"mainDomain"`
	ExtraDomains []string `json:"extraDomains"`

	ListenTo          string `json:"listenTo"`
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
		CreateServerConn: connFactory,
		ConnCh:           connCh,
		CloseCh:          closeCh,
	}
}

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

	hs := conn.HandshakePacket
	if err := serverConn.WritePacket(hs); err != nil {
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
					log.Println("Didnt recognize handshake id")
				}
				if err != nil {
					log.Printf("error login: %v", err)
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
