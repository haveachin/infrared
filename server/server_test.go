package server_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
	"github.com/haveachin/infrared/server"
)

var (
	testHSID    byte = 5
	testLoginID byte = 6

	ErrNotImplemented = errors.New("not implemented")

	defaultChanTimeout = 50 * time.Millisecond
)

type LoginData struct {
	hs         protocol.Packet
	loginStart protocol.Packet
}

func loginClient(conn net.Conn, data LoginData) {
	bytes, _ := data.hs.Marshal()
	conn.Write(bytes)

	bytes, _ = data.loginStart.Marshal()
	conn.Write(bytes)

	//Write something for (optional) pipe logic...?
}

type StatusData struct {
	doPing bool
	pingPk protocol.Packet
	pingCh chan<- protocol.Packet

	hs             protocol.Packet
	request        protocol.Packet
	receivedStatus protocol.Packet
}

func (data *StatusData) statusClient(conn net.Conn) {
	bytes, _ := data.hs.Marshal()
	conn.Write(bytes)

	bytes, _ = data.request.Marshal()
	conn.Write(bytes)

	bufReader := bufio.NewReader(conn)

	data.receivedStatus, _ = protocol.ReadPacket(bufReader)

	if data.doPing {
		pingBytes, _ := data.pingPk.Marshal()
		conn.Write(pingBytes)

		receivedPingPk, _ := protocol.ReadPacket(bufReader)
		data.pingCh <- receivedPingPk
	} else {
		conn.Close()
	}

}

func testSamePK(t *testing.T, expected, received protocol.Packet) {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	if !sameID && sameData {
		t.Logf("expected:\t%v", expected)
		t.Logf("got:\t%v", received)
		t.Error("Received packet is different from what we expected")
	}
}

func shouldStopTest(t *testing.T, err, expectedError error) bool {
	if err != nil && errors.Is(err, expectedError) {
		t.Log("error matched expected error")
		return true
	} else if err != nil {
		t.Log("error didnt match expected error")
		t.Log(err)
		return true
	} else if err == nil && expectedError != nil {
		t.Error("expected an error but didnt got any")
		t.Logf("expected:\t%v", expectedError)
		t.Logf("got:\t\t%v", err)
		return true
	} else {
		return false
	}
}

// Help Methods
func samePK(expected, received protocol.Packet) bool {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	return sameID && sameData
}

// Test themselves
// func TestServerStatusRequest(t *testing.T) {
// 	basicStatus, _ := infrared.StatusConfig{
// 		VersionName:    "Latest",
// 		ProtocolNumber: 1,
// 		MaxPlayers:     999,
// 		MOTD:           "One of a kind server!",
// 	}.StatusResponsePacket()
// 	onlineConfigStatus, _ := infrared.StatusConfig{
// 		VersionName:    "Online",
// 		ProtocolNumber: 2,
// 		MaxPlayers:     998,
// 		MOTD:           "Two of a kind server!",
// 	}.StatusResponsePacket()
// 	offlineConfigStatus, _ := infrared.StatusConfig{
// 		VersionName:    "Offline",
// 		ProtocolNumber: 2,
// 		MaxPlayers:     998,
// 		MOTD:           "Two of a kind server!",
// 	}.StatusResponsePacket()

// 	// TestCase statuses
// 	onlineServerStatus := basicStatus
// 	offlineServerStatus, _ := infrared.StatusConfig{}.StatusResponsePacket()
// 	onlineConfigServerStats := onlineConfigStatus
// 	offlineConfigServerStats := offlineConfigStatus

// 	tt := []struct {
// 		online         bool
// 		configStatus   bool
// 		expectedStatus protocol.Packet
// 	}{
// 		{
// 			online:         true,
// 			configStatus:   false,
// 			expectedStatus: onlineServerStatus,
// 		},
// 		{
// 			online:         false,
// 			configStatus:   false,
// 			expectedStatus: offlineServerStatus,
// 		},
// 		{
// 			online:         true,
// 			configStatus:   true,
// 			expectedStatus: onlineConfigServerStats,
// 		},
// 		{
// 			online:         false,
// 			configStatus:   true,
// 			expectedStatus: offlineConfigServerStats,
// 		},
// 	}

// 	for _, tc := range tt {
// 		name := fmt.Sprintf("online: %v, configStatus: %v", tc.online, tc.configStatus)
// 		t.Run(name, func(t *testing.T) {
// 			wg := &sync.WaitGroup{}
// 			connCh := make(chan connection.GatewayConnection)
// 			sConnMock := testServerConn{status: basicStatus, isOnline: tc.online}
// 			statusFactory := func(addr string) (connection.ServerConnection, error) {
// 				return &sConnMock, nil
// 			}
// 			mcServer := &server.MCServer{
// 				ConnFactory: statusFactory,
// 				ConnCh:      connCh,
// 			}

// 			if tc.configStatus {
// 				mcServer.OnlineConfigStatus = onlineConfigStatus
// 				mcServer.OfflineConfigStatus = offlineConfigStatus
// 			}

// 			go func() {
// 				go mcServer.Start()
// 			}()

// 			statusConn := &testStatusConn{wg: wg}
// 			wg.Add(1)
// 			select {
// 			case connCh <- statusConn:
// 				t.Log("Channel took connection")
// 			case <-time.After(defaultChanTimeout):
// 				t.Log("Tasked timed out")
// 				t.FailNow() // Dont check other code it didnt finish anyway
// 			}

// 			wg.Wait()
// 			receivedPk := statusConn.pk()

// 			if ok := samePK(tc.expectedStatus, receivedPk); !ok {
// 				t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", tc.expectedStatus, receivedPk)
// 			}

// 		})
// 	}

// }

func TestMCServer(t *testing.T) {
	runServer := func(connFactory connection.ServerConnFactory) chan<- connection.HSConnection {
		connCh := make(chan connection.HSConnection)

		mcServer := &server.MCServer{
			ConnFactory: connFactory,
			ConnCh:      connCh,
		}
		go func() {
			mcServer.Start()
		}()

		return connCh
	}
	proxyRequest := false
	proxyPing := false
	testServerLogin(t, runServer)
	testServerStatus_WithoutConfigStatus(t, runServer, proxyRequest, proxyPing)
}

type runTestServer func(connection.ServerConnFactory) chan<- connection.HSConnection

func testServerLogin(t *testing.T, runServer runTestServer) {
	hs := handshaking.ServerBoundHandshake{
		NextState: 2,
	}
	hsPk := hs.Marshal()
	loginPk := protocol.Packet{ID: testLoginID}
	tt := []struct {
		name          string
		hsPk          protocol.Packet
		loginPk       protocol.Packet
		expectedError error
	}{
		{
			name:          "normal run",
			hsPk:          hsPk,
			loginPk:       loginPk,
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}
			loginConn := connection.CreateBasicPlayerConnection(c1, netAddr)
			loginData := LoginData{
				hs:         tc.hsPk,
				loginStart: tc.loginPk,
			}

			go func() {
				loginClient(c2, loginData)
			}()

			s1, s2 := net.Pipe()
			sConn := connection.CreateBasicServerConn(s1)

			connFactory := func(addr string) (connection.ServerConnection, error) {
				return sConn, nil
			}

			connCh := runServer(connFactory)

			select {
			case connCh <- loginConn:
				t.Log("Channel took connection")
			case <-time.After(defaultChanTimeout):
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			}
			bufferReader := bufio.NewReader(s2)

			receivedHsPk, err := protocol.ReadPacket(bufferReader)
			if shouldStopTest(t, err, tc.expectedError) {
				t.Skip()
			}
			testSamePK(t, tc.hsPk, receivedHsPk)

			receivedLoginPk, err := protocol.ReadPacket(bufferReader)
			if shouldStopTest(t, err, tc.expectedError) {
				t.Skip()
			}
			testSamePK(t, tc.loginPk, receivedLoginPk)

			// a little pipe testing?
		})
	}
}

type serverStatusTestcase struct {
	name          string
	expectedError error // Not sure how to error between connections

	shouldNOTFinish                  bool
	cutConnBeforeSendingServerStatus bool

	hsPk             protocol.Packet
	requestPk        protocol.Packet
	expectedStatusPk protocol.Packet

	doPing     bool
	pingPacket protocol.Packet
}

// Didnt confirm or proxyPing test works, proxy request test does work.
//  Please remove this after its confirmed that it does work
func testServerStatus_WithoutConfigStatus(t *testing.T, runServer runTestServer, proxyRequest, proxyPing bool) {

	hs := handshaking.ServerBoundHandshake{
		ProtocolVersion: 754,
		ServerAddress:   "infrared",
		ServerPort:      25565,
		NextState:       1,
	}
	hsPk := hs.Marshal()
	responseJSON := status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     "infrared",
			Protocol: 754,
		},
		Description: status.DescriptionJSON{
			Text: "Random status",
		},
	}

	bb, _ := json.Marshal(responseJSON)
	normalStatus := status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()

	emptyStatus := status.ClientBoundResponse{}.Marshal()
	normalRequestPk := protocol.Packet{ID: 0x00}
	specialRequestPk := protocol.Packet{ID: 0x12}

	normalPingPk := protocol.Packet{ID: 0x01}
	specialPingPk := protocol.Packet{ID: 0x10}
	tt := []serverStatusTestcase{
		{
			name:             "normal run without ping",
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: normalStatus,
		},
		{
			name:             "normal run with ping",
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: normalStatus,
			doPing:           true,
			pingPacket:       normalPingPk,
		},
		{
			name:                             "cut connection instead of sending server status without ping",
			hsPk:                             hsPk,
			requestPk:                        normalRequestPk,
			expectedStatusPk:                 emptyStatus,
			shouldNOTFinish:                  false,
			cutConnBeforeSendingServerStatus: true,
		},
		{
			name:                             "cut connection instead of sending server status with ping",
			hsPk:                             hsPk,
			requestPk:                        normalRequestPk,
			expectedStatusPk:                 emptyStatus,
			doPing:                           true,
			pingPacket:                       normalPingPk,
			shouldNOTFinish:                  false,
			cutConnBeforeSendingServerStatus: true,
		},
	}

	if proxyPing {
		tt = append(tt, serverStatusTestcase{
			name:             "different ping packet",
			doPing:           true,
			pingPacket:       specialPingPk,
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: emptyStatus,
		})
	}

	if proxyRequest {
		tt = append(tt, serverStatusTestcase{
			name:             "different request packet without ping",
			hsPk:             hsPk,
			requestPk:        specialRequestPk,
			expectedStatusPk: emptyStatus,
		}, serverStatusTestcase{
			name:             "different request packet with ping",
			hsPk:             hsPk,
			requestPk:        specialRequestPk,
			expectedStatusPk: emptyStatus,
			doPing:           true,
			pingPacket:       normalPingPk,
		})
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}
			statusConn := connection.CreateBasicPlayerConnection(c1, netAddr)
			pingCh := make(chan protocol.Packet)
			statusData := &StatusData{
				doPing:  tc.doPing,
				pingCh:  pingCh,
				pingPk:  tc.pingPacket,
				hs:      tc.hsPk,
				request: tc.requestPk,
			}

			go func() {
				statusData.statusClient(c2)
			}()

			s1, s2 := net.Pipe()
			sConn := connection.CreateBasicServerConn(s1)

			connFactory := func(addr string) (connection.ServerConnection, error) {
				return sConn, nil
			}

			connCh := runServer(connFactory)

			select {
			case connCh <- statusConn:
				t.Log("Channel took connection")
			case <-time.After(defaultChanTimeout):
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			}

			bufReader := bufio.NewReader(s2)

			receivedHsPk, err := protocol.ReadPacket(bufReader)
			if shouldStopTest(t, err, tc.expectedError) {
				t.Skip()
			}
			testSamePK(t, tc.hsPk, receivedHsPk)

			receivedLoginPk, err := protocol.ReadPacket(bufReader)
			if shouldStopTest(t, err, tc.expectedError) {
				t.Skip()
			}
			testSamePK(t, tc.requestPk, receivedLoginPk)

			if tc.cutConnBeforeSendingServerStatus {
				s2.Close()
			} else {
				statusBytes, _ := tc.expectedStatusPk.Marshal()
				s2.Write(statusBytes)
			}

			testSamePK(t, tc.expectedStatusPk, statusData.receivedStatus)

			if tc.doPing {
				if proxyPing {
					receivedPingPk, err := protocol.ReadPacket(bufReader)
					if shouldStopTest(t, err, tc.expectedError) {
						t.Skip()
					}
					testSamePK(t, tc.pingPacket, receivedPingPk)

					responseBytes, _ := receivedPingPk.Marshal()
					s2.Write(responseBytes)

					testSamePK(t, tc.pingPacket, receivedPingPk)

				} else {
					receivedPingPk := <-pingCh
					testSamePK(t, tc.pingPacket, receivedPingPk)
				}
			}

			if tc.shouldNOTFinish {
				t.Error("finished with getting status but was supposed to fail")
			}

		})
	}
}
