package server_test

import (
	"bytes"
	"errors"
	"fmt"
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
