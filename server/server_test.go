package server_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/server"
)

type StatusError struct {
	expected infrared.StatusConfig
	received infrared.StatusConfig
}

// Data types
type testServerConn struct {
	status             protocol.Packet
	isOnline           bool
	receivedHandshake  bool
	receivedLoginStart bool
}

func (conn *testServerConn) Status() (protocol.Packet, error) {
	if conn.isOnline {
		return conn.status, nil
	}
	// Sending status anyway on purpose
	return conn.status, server.ErrCantConnectWithServer
}

var (
	testHSID    byte = 5
	testLoginID byte = 6
)

func (conn *testServerConn) SendPK(pk protocol.Packet) error {
	switch pk.ID {
	case testHSID:
		conn.receivedHandshake = true
	case testLoginID:
		conn.receivedLoginStart = true
	}
	return nil
}

func (conn *testServerConn) ReceivedHandshake() bool {
	return conn.receivedHandshake
}

func (conn *testServerConn) ReceivedLoginStart() bool {
	return conn.receivedLoginStart
}

type testStatusConn struct {
	status protocol.Packet
}

func (conn *testStatusConn) SendStatus(pk protocol.Packet) error {
	conn.status = pk
	return nil
}

func (conn *testStatusConn) pk() protocol.Packet {
	return conn.status
}

type testLoginConn struct {
}

func (conn testLoginConn) HS() (protocol.Packet, error) {
	return protocol.Packet{ID: testHSID}, nil
}

func (conn testLoginConn) LoginStart() (protocol.Packet, error) {
	return protocol.Packet{ID: testLoginID}, nil
}

// Help Methods
func samePK(expected, received protocol.Packet) bool {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	return sameID && sameData
}

// Test themselves
func TestHandleStatusRequest(t *testing.T) {
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
			sConnMock := testServerConn{status: basicStatus, isOnline: tc.online}
			statusFactory := func() connection.ServerConnection {
				return &sConnMock
			}
			mcServer := &server.MCServer{
				ConnFactory:         statusFactory,
				OnlineConfigStatus:  onlineConfigStatus,
				OfflineConfigStatus: offlineConfigStatus,
				UseConfigStatus:     tc.configStatus,
			}

			statusConn := &testStatusConn{}

			err := server.HandleStatusRequest(statusConn, mcServer)
			if err != nil {
				t.Errorf("got error %v", err)
			}

			receivedPk := statusConn.pk()

			if ok := samePK(tc.expectedStatus, receivedPk); !ok {
				t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", tc.expectedStatus, receivedPk)
			}

		})
	}

}

func TestHandleLoginRequest(t *testing.T) {
	sConnMock := testServerConn{}
	statusFactory := func() connection.ServerConnection {
		return &sConnMock
	}
	loginConn := &testLoginConn{}
	mcServer := &server.MCServer{ConnFactory: statusFactory}

	server.HandleLoginRequest(loginConn, mcServer)

	ok := sConnMock.ReceivedHandshake()
	if !ok {
		t.Error("Didnt receive handshake")
	}

	ok = sConnMock.ReceivedLoginStart()
	if !ok {
		t.Error("Didnt receive Login Start packet")
	}

}
