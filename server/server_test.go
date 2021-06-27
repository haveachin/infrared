package server_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/login"
	"github.com/haveachin/infrared/protocol/status"
	"github.com/haveachin/infrared/server"
)

var (
	testLoginID byte = 6

	ErrNotImplemented = errors.New("not implemented")

	defaultChTimeout = 10 * time.Millisecond
)

type LoginData struct {
	hs         protocol.Packet
	loginStart protocol.Packet
}

func loginClient(conn net.Conn, data LoginData) {
	bytes, _ := data.loginStart.Marshal()
	conn.Write(bytes)

	//Write something for (optional) pipe logic...?
}

type StatusData struct {
	doPing bool
	pingPk protocol.Packet
	pingCh chan<- protocol.Packet

	hsPk           protocol.Packet
	request        protocol.Packet
	receivedStatus protocol.Packet
}

func (data *StatusData) statusClient(conn net.Conn) {
	bytes, _ := data.request.Marshal()
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

type serverStatusRequestData struct {
	expectedOnlineStatus  protocol.Packet
	expectedOfflineStatus protocol.Packet
	serverStatusResponse  protocol.Packet

	server func(net.Conn) server.MCServer
}

func testServerStatusRequest(t *testing.T, testData serverStatusRequestData) {
	tt := []struct {
		online         bool
		configStatus   bool
		expectedStatus protocol.Packet
	}{
		{
			online:         true,
			expectedStatus: testData.expectedOnlineStatus,
		},
		{
			online:         false,
			expectedStatus: testData.expectedOfflineStatus,
		},
	}

	for _, tc := range tt {
		name := fmt.Sprintf("online: %v, configStatus: %v", tc.online, tc.configStatus)
		t.Run(name, func(t *testing.T) {
			s1, s2 := net.Pipe()
			mcServer := testData.server(s1)

			go func() {
				if !tc.online {
					s2.Close()
					return
				}
				serverConn2 := connection.NewServerConn(s2)
				serverConn2.ReadPacket()
				serverConn2.ReadPacket()
				serverConn2.WritePacket(testData.serverStatusResponse)
			}()

			hs := handshaking.ServerBoundHandshake{}
			hsPk := hs.Marshal()
			statusConn := connection.HandshakeConn{}
			statusConn.HandshakePacket = hsPk

			receivedPk := mcServer.Status(statusConn)

			if ok := samePK(tc.expectedStatus, receivedPk); !ok {
				t.Logf("expected:\t%v", tc.expectedStatus)
				t.Logf("got:\t\t%v", receivedPk)
				t.Error("Received packet is different from what we expected")
			}
		})
	}

}

func TestMCServer(t *testing.T) {
	runServer := func(connFactory connection.ServerConnFactory) chan<- connection.HandshakeConn {
		connCh := make(chan connection.HandshakeConn)

		mcServer := &server.MCServer{
			CreateServerConn: connFactory,
			ConnCh:           connCh,
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

	onlineServerStatus := basicStatus
	offlineServerStatus, _ := infrared.StatusConfig{}.StatusResponsePacket()

	statusFactory := func(conn net.Conn) server.MCServer {
		serverConn := connection.NewServerConn(conn)
		statusFactory := func() (connection.ServerConn, error) {
			return serverConn, nil
		}

		mcServer := server.MCServer{
			CreateServerConn: statusFactory,
		}
		return mcServer
	}
	statusServerData := serverStatusRequestData{
		expectedOnlineStatus:  onlineServerStatus,
		expectedOfflineStatus: offlineServerStatus,
		serverStatusResponse:  basicStatus,
		server:                statusFactory,
	}
	testServerStatusRequest(t, statusServerData)

	statusConfigFactory := func(conn net.Conn) server.MCServer {
		serverConn := connection.NewServerConn(conn)
		statusFactory := func() (connection.ServerConn, error) {
			return serverConn, nil
		}

		mcServer := server.MCServer{
			CreateServerConn:    statusFactory,
			OnlineConfigStatus:  onlineConfigStatus,
			OfflineConfigStatus: offlineConfigStatus,
		}
		return mcServer
	}
	statusServerData_Config := serverStatusRequestData{
		expectedOnlineStatus:  onlineConfigStatus,
		expectedOfflineStatus: offlineConfigStatus,
		serverStatusResponse:  basicStatus,
		server:                statusConfigFactory,
	}
	testServerStatusRequest(t, statusServerData_Config)

}

type runTestServer func(connection.ServerConnFactory) chan<- connection.HandshakeConn

func testServerLogin(t *testing.T, runServer runTestServer) {
	hs := handshaking.ServerBoundHandshake{
		NextState: 2,
	}
	hsPk := hs.Marshal()
	loginPk := protocol.Packet{ID: testLoginID}
	tt := []struct {
		name          string
		hs            handshaking.ServerBoundHandshake
		hsPk          protocol.Packet
		loginPk       protocol.Packet
		expectedError error
	}{
		{
			name:          "normal run",
			hs:            hs,
			hsPk:          hsPk,
			loginPk:       loginPk,
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}
			loginConn := connection.NewHandshakeConn(c1, netAddr)
			loginData := LoginData{
				hs:         tc.hsPk,
				loginStart: tc.loginPk,
			}
			loginConn.Handshake = tc.hs
			loginConn.HandshakePacket = tc.hsPk
			go func() {
				loginClient(c2, loginData)
			}()

			s1, s2 := net.Pipe()
			sConn := connection.NewServerConn(s1)

			connFactory := func() (connection.ServerConn, error) {
				return sConn, nil
			}

			connCh := runServer(connFactory)

			select {
			case connCh <- loginConn:
				t.Log("Server took connection")
			case <-time.After(defaultChTimeout):
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

			// a little pipe testing here?
		})
	}
}

type serverStatusTestcase struct {
	name          string
	expectedError error // Not sure how to error between connections

	shouldNOTFinish                  bool
	cutConnBeforeSendingServerStatus bool

	hs               handshaking.ServerBoundHandshake
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
			hs:               hs,
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: normalStatus,
		},
		{
			name:             "normal run with ping",
			hs:               hs,
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: normalStatus,
			doPing:           true,
			pingPacket:       normalPingPk,
		},
		{
			name:                             "cut connection instead of sending server status without ping",
			hs:                               hs,
			hsPk:                             hsPk,
			requestPk:                        normalRequestPk,
			expectedStatusPk:                 emptyStatus,
			shouldNOTFinish:                  false,
			cutConnBeforeSendingServerStatus: true,
		},
		{
			name:                             "cut connection instead of sending server status with ping",
			hs:                               hs,
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
			hs:               hs,
			hsPk:             hsPk,
			requestPk:        normalRequestPk,
			expectedStatusPk: emptyStatus,
		})
	}

	if proxyRequest {
		tt = append(tt, serverStatusTestcase{
			name:             "different request packet without ping",
			hs:               hs,
			hsPk:             hsPk,
			requestPk:        specialRequestPk,
			expectedStatusPk: emptyStatus,
		}, serverStatusTestcase{
			name:             "different request packet with ping",
			hs:               hs,
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
			statusConn := connection.NewHandshakeConn(c1, netAddr)
			pingCh := make(chan protocol.Packet)
			statusData := &StatusData{
				doPing:  tc.doPing,
				pingCh:  pingCh,
				pingPk:  tc.pingPacket,
				hsPk:    tc.hsPk,
				request: tc.requestPk,
			}
			statusConn.Handshake = tc.hs
			statusConn.HandshakePacket = tc.hsPk

			go func() {
				statusData.statusClient(c2)
			}()

			s1, s2 := net.Pipe()
			sConn := connection.NewServerConn(s1)

			connFactory := func() (connection.ServerConn, error) {
				return sConn, nil
			}

			connCh := runServer(connFactory)

			select {
			case connCh <- statusConn:
				t.Log("Server took connection")
			case <-time.After(defaultChTimeout):
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

func TestOrMCServerCanClose(t *testing.T) {
	closeCh := make(chan struct{})
	connCh := make(chan connection.HandshakeConn)
	handshakeConn := connection.NewHandshakeConn(nil, nil)

	server := server.MCServer{
		ConnCh:  connCh,
		CloseCh: closeCh,
	}
	go func() {
		server.Start()
	}()

	closeCh <- struct{}{}
	select {
	case <-time.After(defaultChTimeout):
		t.Log("Everything is fine the task timed out like it should have")
	case connCh <- handshakeConn:
		t.Log("Tasked should have timed out")
		t.FailNow()
	}
}

func TestMCServerErrorDetection(t *testing.T) {
	statusHandshake := handshaking.ServerBoundHandshake{
		NextState: 1,
	}
	loginHandshake := handshaking.ServerBoundHandshake{
		NextState: 2,
	}
	tt := []struct {
		handshake              handshaking.ServerBoundHandshake
		closeClientAfterReadN  int
		closeClientAfterWriteN int
		closeServerAfterReadN  int
		closeServerAfterWriteN int
	}{
		{
			handshake:              loginHandshake,
			closeServerAfterReadN:  -1,
			closeServerAfterWriteN: -1,
		},
		{
			handshake:              loginHandshake,
			closeServerAfterWriteN: -1,
		},
		{
			handshake:              loginHandshake,
			closeClientAfterWriteN: -1,
		},
		{
			handshake:             loginHandshake,
			closeServerAfterReadN: 1,
		},
		{
			handshake:             loginHandshake,
			closeServerAfterReadN: 2,
		},
		{
			handshake:              loginHandshake,
			closeClientAfterWriteN: 1,
		},

		{
			handshake:              statusHandshake,
			closeClientAfterWriteN: -1,
		},
		{
			handshake:              statusHandshake,
			closeClientAfterWriteN: 1,
		},
		{
			handshake:             statusHandshake,
			closeClientAfterReadN: 1,
		},
		{
			handshake:              statusHandshake,
			closeClientAfterWriteN: 2,
		},
		{
			handshake:             statusHandshake,
			closeClientAfterReadN: 2,
		},
	}

	for _, tc := range tt {
		name := fmt.Sprintf("hs-state:%v, Client:%d-%d, Server:%d-%d (read-write)", tc.handshake.NextState, tc.closeClientAfterReadN, tc.closeClientAfterWriteN, tc.closeServerAfterReadN, tc.closeServerAfterWriteN)
		t.Run(name, func(t *testing.T) {
			closeCh := make(chan struct{})
			connCh := make(chan connection.HandshakeConn)
			c1, c2 := net.Pipe()
			s1, s2 := net.Pipe()
			go serverThing(s2, int(tc.handshake.NextState), tc.closeServerAfterReadN, tc.closeServerAfterWriteN)
			go clientThing(c2, int(tc.handshake.NextState), tc.closeClientAfterReadN, tc.closeClientAfterWriteN)
			handshakeConn := connection.NewHandshakeConn(c1, nil)
			handshakeConn.Handshake = tc.handshake
			handshakeConn.HandshakePacket = tc.handshake.Marshal()
			var serverConn connection.ServerConnFactory

			if tc.closeServerAfterReadN == -1 && tc.closeServerAfterWriteN == -1 {
				serverConn = func() (connection.ServerConn, error) {
					return connection.ServerConn{}, &net.OpError{Op: "dial", Net: "0.0.0.0", Source: nil, Addr: nil, Err: errors.New("connfection refused")}
				}
			} else {
				serverConn = func() (connection.ServerConn, error) {
					return connection.NewServerConn(s1), nil
				}
			}

			go func() {
				server := server.MCServer{
					CreateServerConn: serverConn,
					ConnCh:           connCh,
					CloseCh:          closeCh,
				}
				server.Start()
			}()
			connCh <- handshakeConn

			time.Sleep(2 * defaultChTimeout)
			// Client connections needs to be broken by any error expect for server not being able to be reached (status:)
			_, err := c2.Write([]byte{1, 2, 3, 4, 5})
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Log(err)
				t.Fail()
			}
		})
	}
}

func serverThing(conn net.Conn, state, closeAfterReadN, closeAfterWriteN int) {
	readBuffer := make([]byte, 25565)
	switch state {
	case 1: // Cancelling this should return offline status to client not closing its connection
		// Receives handshake
		conn.Read(readBuffer)
		// Receives status request
		conn.Read(readBuffer)
		pkStatus := status.ClientBoundResponse{}.Marshal()
		bytes, _ := pkStatus.Marshal()
		conn.Write(bytes)
	case 2:
		if closeAfterWriteN == -1 || closeAfterReadN == -1 {
			conn.Close()
			return
		}
		//Read Handshake
		conn.Read(readBuffer)
		if closeAfterReadN == 1 {
			conn.Close()
			return
		}
		//Read LoginRequest packet
		conn.Read(readBuffer)
		if closeAfterReadN == 2 {
			conn.Close()
			return
		}
		// Here starts the pipe connection
		couldReadCh := make(chan struct{})
		go func() {
			conn.Read(readBuffer)
			couldReadCh <- struct{}{}
		}()

		select {
		case <-couldReadCh:
		case <-time.After(defaultChTimeout):
			conn.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
		}

	}
}

func clientThing(conn net.Conn, state, closeAfterReadN, closeAfterWriteN int) {
	readBuffer := make([]byte, 0xffff)
	switch state {
	case 1:
		if closeAfterWriteN == -1 || closeAfterReadN == -1 {
			conn.Close()
			return
		}
		pkStatus := status.ServerBoundRequest{}.Marshal()
		bytes, _ := pkStatus.Marshal()
		conn.Write(bytes)
		if closeAfterWriteN == 1 {
			conn.Close()
			return
		}
		conn.Read(readBuffer)
		if closeAfterReadN == 1 {
			conn.Close()
			return
		}
		pkPing := protocol.Packet{ID: 0x01, Data: []byte{8}}
		bytes, _ = pkPing.Marshal()
		conn.Write(bytes)
		if closeAfterWriteN == 2 {
			conn.Close()
			return
		}
		conn.Read(readBuffer)
		if closeAfterReadN == 2 {
			conn.Close()
			return
		}
	case 2:
		if closeAfterWriteN == -1 || closeAfterReadN == -1 {
			conn.Close()
			return
		}
		pkLogin := login.ServerLoginStart{Name: "Infrared"}.Marshal()
		bytes, _ := pkLogin.Marshal()
		conn.Write(bytes)
		if closeAfterWriteN == 1 {
			conn.Close()
			return
		}

		//Here starts pipe connection
		couldWriteCh := make(chan struct{})
		go func() {
			conn.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
			couldWriteCh <- struct{}{}
		}()

		select {
		case <-couldWriteCh:
		case <-time.After(defaultChTimeout):
			conn.Read(readBuffer)
		}
	}
}
