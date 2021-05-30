package server_test

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/server"
)

var (
	testHSID    byte = 5
	testLoginID byte = 6

	ErrNotImplemented = errors.New("not implemented")
)

// Data types
type testServerConn struct {
	status             protocol.Packet
	isOnline           bool
	receivedHandshake  bool
	receivedLoginStart bool

	wg *sync.WaitGroup
}

func (conn *testServerConn) Status(pk protocol.Packet) (protocol.Packet, error) {
	if conn.isOnline {
		return conn.status, nil
	}
	// Sending status anyway on purpose
	return conn.status, server.ErrCantConnectWithServer
}

func (conn *testServerConn) SendPK(pk protocol.Packet) error {
	switch pk.ID {
	case handshaking.ServerBoundHandshakePacketID:
		conn.receivedHandshake = true
	case testLoginID:
		conn.receivedLoginStart = true
	}
	conn.wg.Done()
	return nil
}

func (conn *testServerConn) ReceivedHandshake() bool {
	return conn.receivedHandshake
}

func (conn *testServerConn) ReceivedLoginStart() bool {
	return conn.receivedLoginStart
}

func (c *testServerConn) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *testServerConn) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

type testStatusConn struct {
	status protocol.Packet
	wg     *sync.WaitGroup
}

func (conn *testStatusConn) SendStatus(pk protocol.Packet) error {
	conn.status = pk
	return nil
}

func (conn *testStatusConn) pk() protocol.Packet {
	return conn.status
}

func (conn *testStatusConn) Hs() (handshaking.ServerBoundHandshake, error) {
	return handshaking.ServerBoundHandshake{NextState: 1}, nil
}

func (conn *testStatusConn) HsPk() (protocol.Packet, error) {
	return protocol.Packet{ID: testHSID}, nil
}

func (conn *testStatusConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (conn *testStatusConn) ReadPacket() (protocol.Packet, error) {
	return protocol.Packet{}, nil
}

func (conn *testStatusConn) WritePacket(p protocol.Packet) error {
	conn.status = p
	conn.wg.Done()
	return nil
}

func (c *testStatusConn) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *testStatusConn) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *testStatusConn) ServerAddr() string {
	return ""
}

type testLoginConn struct {
	hs      handshaking.ServerBoundHandshake
	loginPK protocol.Packet
}

func (conn testLoginConn) Hs() (handshaking.ServerBoundHandshake, error) {
	return conn.hs, nil
}

func (conn testLoginConn) HsPk() (protocol.Packet, error) {
	return conn.hs.Marshal(), nil
}

func (conn testLoginConn) LoginStart() (protocol.Packet, error) {
	return conn.loginPK, nil
}

func (conn testLoginConn) RequestType() connection.RequestType {
	return connection.RequestType(conn.hs.NextState)
}

func (conn testLoginConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (conn testLoginConn) ReadPacket() (protocol.Packet, error) {
	return protocol.Packet{}, nil
}

func (conn testLoginConn) WritePacket(p protocol.Packet) error {
	return nil
}

func (conn testLoginConn) Name() (string, error) {
	return "", nil
}

func (c testLoginConn) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c testLoginConn) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c testLoginConn) ServerAddr() string {
	return string(c.hs.ServerAddress)
}

// Help Methods
func samePK(expected, received protocol.Packet) bool {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	return sameID && sameData
}

// Test themselves
func TestServerStatusRequest(t *testing.T) {
	basicStatus, _ := infrared.StatusConfig{
		VersionName:    "Latest",
		ProtocolNumber: 1,
		MaxPlayers:     999,
		MOTD:           "One of a kind server!",
	}.StatusResponsePacket()
	onlineConfigStatus, _ := infrared.StatusConfig{
		VersionName:    "Online",
		ProtocolNumber: 2,
		MaxPlayers:     998,
		MOTD:           "Two of a kind server!",
	}.StatusResponsePacket()
	offlineConfigStatus, _ := infrared.StatusConfig{
		VersionName:    "Offline",
		ProtocolNumber: 2,
		MaxPlayers:     998,
		MOTD:           "Two of a kind server!",
	}.StatusResponsePacket()

	// TestCase statuses
	onlineServerStatus := basicStatus
	offlineServerStatus, _ := infrared.StatusConfig{}.StatusResponsePacket()
	onlineConfigServerStats := onlineConfigStatus
	offlineConfigServerStats := offlineConfigStatus

	tt := []struct {
		online         bool
		configStatus   bool
		expectedStatus protocol.Packet
	}{
		{
			online:         true,
			configStatus:   false,
			expectedStatus: onlineServerStatus,
		},
		{
			online:         false,
			configStatus:   false,
			expectedStatus: offlineServerStatus,
		},
		{
			online:         true,
			configStatus:   true,
			expectedStatus: onlineConfigServerStats,
		},
		{
			online:         false,
			configStatus:   true,
			expectedStatus: offlineConfigServerStats,
		},
	}

	for _, tc := range tt {
		name := fmt.Sprintf("online: %v, configStatus: %v", tc.online, tc.configStatus)
		t.Run(name, func(t *testing.T) {
			wg := &sync.WaitGroup{}
			connCh := make(chan connection.GatewayConnection)
			sConnMock := testServerConn{status: basicStatus, isOnline: tc.online}
			statusFactory := func(addr string) (connection.ServerConnection, error) {
				return &sConnMock, nil
			}
			mcServer := &server.MCServer{
				ConnFactory: statusFactory,
				ConnCh:      connCh,
			}

			if tc.configStatus {
				mcServer.OnlineConfigStatus = onlineConfigStatus
				mcServer.OfflineConfigStatus = offlineConfigStatus
			}

			go func() {
				go mcServer.Start()
			}()

			statusConn := &testStatusConn{wg: wg}
			wg.Add(1)
			select {
			case connCh <- statusConn:
				t.Log("Channel took connection")
			case <-time.After(1 * time.Millisecond):
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			}

			wg.Wait()
			receivedPk := statusConn.pk()

			if ok := samePK(tc.expectedStatus, receivedPk); !ok {
				t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", tc.expectedStatus, receivedPk)
			}

		})
	}

}

func TestMCServer_LoginRequest(t *testing.T) {
	runServer := func(connFactory connection.ServerConnFactory) chan<- connection.GatewayConnection {
		connCh := make(chan connection.GatewayConnection)

		mcServer := &server.MCServer{
			ConnFactory: connFactory,
			ConnCh:      connCh,
		}
		go func() {
			mcServer.Start()
		}()

		return connCh
	}

	testServerLoginRequest(t, runServer)
}

func testServerLoginRequest(t *testing.T, runServer func(connection.ServerConnFactory) chan<- connection.GatewayConnection) {
	hs := handshaking.ServerBoundHandshake{
		NextState: 2,
	}
	loginPk := protocol.Packet{ID: testLoginID}

	wg := sync.WaitGroup{}
	wg.Add(2)
	loginConn := &testLoginConn{hs: hs, loginPK: loginPk}
	sConnMock := testServerConn{wg: &wg}
	connFactory := func(addr string) (connection.ServerConnection, error) {
		return &sConnMock, nil
	}

	connCh := runServer(connFactory)

	select {
	case connCh <- loginConn:
		t.Log("Channel took connection")
	case <-time.After(1 * time.Millisecond):
		t.Log("Tasked timed out")
		t.FailNow() // Dont check other code it didnt finish anyway
	}

	wg.Wait()

	ok := sConnMock.ReceivedHandshake()
	if !ok {
		t.Error("Didnt receive handshake")
	}

	ok = sConnMock.ReceivedLoginStart()
	if !ok {
		t.Error("Didnt receive Login Start packet")
	}

}
