package connection_test

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/login"
	"github.com/haveachin/infrared/protocol/status"
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

type statusData struct {
	withPing bool
	hs       protocol.Packet
	status   protocol.Packet
}

func statusClient(conn net.Conn, data statusData) {
	bytes, _ := data.hs.Marshal()
	conn.Write(bytes)

	bytes, _ = data.status.Marshal()
	conn.Write(bytes)

	responseData := make([]byte, 0xffff)
	conn.Read(responseData)

	if data.withPing {
		pingPacket := protocol.Packet{ID: 1}
		pingBytes, _ := pingPacket.Marshal()
		conn.Write(pingBytes)

		pingData := make([]byte, 2)
		conn.Read(pingData)
	}

}

type testMCServer struct {
	receivedHSPk      []byte
	receivedRequestPk []byte

	status []byte

	doPing bool
}

func (s *testMCServer) statusServer(conn net.Conn) {
	s.receivedHSPk = make([]byte, 0xffff)
	conn.Read(s.receivedHSPk)

	s.receivedRequestPk = make([]byte, 0xffff)
	conn.Read(s.receivedRequestPk)

	conn.Write(s.status)

	if s.doPing {
		pingData := make([]byte, 20)
		conn.Read(pingData)

		conn.Write(pingData)
	}

}

func testSameHs(t *testing.T, received, expected handshaking.ServerBoundHandshake) {
	sameAddr := received.ServerAddress == expected.ServerAddress
	samePort := received.ServerPort == expected.ServerPort
	sameNextState := received.NextState == expected.NextState
	sameVersion := received.ProtocolVersion == expected.ProtocolVersion

	sameHs := sameAddr && samePort && sameNextState && sameVersion

	if !sameHs {
		t.Logf("expected:\t%v", expected)
		t.Logf("got:\t\t%v", received)
		t.Error("Received packet is different from what we expected")
	}
}

func testSamePK(t *testing.T, expected, received protocol.Packet) {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	if !sameID && sameData {
		t.Logf("expected:\t%v", expected)
		t.Logf("got:\t\t%v", received)
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

func TestBasicPlayerConnection(t *testing.T) {

	innerFactory := func(conn net.Conn, addr net.Addr) *connection.BasicPlayerConnection {
		return connection.CreateBasicPlayerConnection2(conn, addr)
	}

	loginFactory := func(conn net.Conn, addr net.Addr) connection.LoginConnection {
		return innerFactory(conn, addr)
	}
	testLoginConnection(loginFactory, t)

	hsFactory := func(conn net.Conn, addr net.Addr) connection.HSConnection {
		return innerFactory(conn, addr)
	}
	testHSConnection(hsFactory, t)

	pipeFactory := func(conn net.Conn) connection.PipeConnection {
		return innerFactory(conn, &net.IPAddr{})
	}
	testPipeConnection(pipeFactory, t)
}

type loginConnFactory func(net.Conn, net.Addr) connection.LoginConnection

type loginConnTestCase struct {
	name             string
	loginStartPacket protocol.Packet
	expectedError    error
	mcName           string
}

func testLoginConnection(factory loginConnFactory, t *testing.T) {
	// Were arent testing handshake here thats why there only is a valid one
	hs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25665,
		ProtocolVersion: 765,
		NextState:       2,
	}

	validLoginPacket := login.ServerLoginStart{Name: "Drago"}.Marshal()
	invalidLoginPacket_ID := protocol.Packet{ID: 0x7F, Data: validLoginPacket.Data}
	loginPacket_NameToLong := login.ServerLoginStart{Name: "12345678901234567890"}.Marshal()

	tt := []loginConnTestCase{
		{
			name:             "no error run",
			loginStartPacket: validLoginPacket,
			mcName:           "Drago",
			expectedError:    nil,
		},
		{
			name:             "name too long",
			loginStartPacket: loginPacket_NameToLong,
			mcName:           "12345678901234567890",
			expectedError:    nil, //Doesnt expect this to be a point but maybe in the future
		},
	}

	testLoginConnFactory := func(tc loginConnTestCase) connection.LoginConnection {
		c1, c2 := net.Pipe()

		netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}

		loginConn := factory(c1, netAddr)

		loginData := LoginData{
			hs:         hs.Marshal(),
			loginStart: tc.loginStartPacket,
		}

		go func() {
			loginClient(c2, loginData)
		}()

		loginConn.HsPk() // make it read the handshake
		return loginConn
	}

	func() {
		for _, tc := range tt {
			t.Run("Packet: "+tc.name, func(t *testing.T) {
				if errors.Is(tc.expectedError, protocol.ErrInvalidPacketID) {
					t.Skip() // Need to some either on the conn side or test side
				}
				loginConn := testLoginConnFactory(tc)

				pk, err := loginConn.LoginStart()
				if shouldStopTest(t, err, tc.expectedError) {
					t.Skip()
				}
				testSamePK(t, tc.loginStartPacket, pk)
			})
		}
	}()
	func() {
		tt = append(tt, loginConnTestCase{
			name:             "invalid packet ID",
			loginStartPacket: invalidLoginPacket_ID,
			mcName:           "Drago",
			expectedError:    protocol.ErrInvalidPacketID,
		})
		for _, tc := range tt {
			t.Run("Name: "+tc.name, func(t *testing.T) {
				loginConn := testLoginConnFactory(tc)
				name, err := loginConn.Name()
				if shouldStopTest(t, err, tc.expectedError) {
					t.Skip()
				}

				if tc.mcName != name {
					t.Logf("expected:\t%v", tc.mcName)
					t.Logf("got:\t\t%v", name)
					t.Error("Received packet is different from what we expected")
				}
			})
		}
	}()

}

type hsConnFactory func(net.Conn, net.Addr) connection.HSConnection

type hsConnTestCase struct {
	name          string
	hs            handshaking.ServerBoundHandshake
	hsPacket      protocol.Packet
	expectedError error
}

func testHSConnection(factory hsConnFactory, t *testing.T) {
	// Were arent testing handshake here thats why there only is a valid one
	validLoginHs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25665,
		ProtocolVersion: 765,
		NextState:       2,
	}
	validHSPacket := validLoginHs.Marshal()
	invalidLoginHs := protocol.Packet{ID: 0x7F, Data: validHSPacket.Data}

	tt := []hsConnTestCase{
		{
			name:          "no error run",
			hs:            validLoginHs,
			hsPacket:      validHSPacket,
			expectedError: nil,
		},
	}

	testHSConnFactory := func(tc hsConnTestCase) connection.HSConnection {
		c1, c2 := net.Pipe()

		netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}

		hsConn := factory(c1, netAddr)

		loginData := LoginData{
			hs: tc.hsPacket,
		}

		go func() {
			loginClient(c2, loginData)
		}()

		return hsConn
	}

	func() {
		for _, tc := range tt {
			t.Run("Packet: "+tc.name, func(t *testing.T) {
				if errors.Is(tc.expectedError, protocol.ErrInvalidPacketID) {
					t.Skip() // Need to some either on the conn side or test side
				}
				hsConn := testHSConnFactory(tc)

				pk, err := hsConn.HsPk()
				if shouldStopTest(t, err, tc.expectedError) {
					t.Skip()
				}
				testSamePK(t, tc.hsPacket, pk)
			})
		}
	}()

	func() {
		tt = append(tt, hsConnTestCase{
			name:          "invalid packet ID",
			hsPacket:      invalidLoginHs,
			expectedError: protocol.ErrInvalidPacketID,
		})
		for _, tc := range tt {
			t.Run("Name: "+tc.name, func(t *testing.T) {
				hsConn := testHSConnFactory(tc)
				hs, err := hsConn.Handshake()
				if shouldStopTest(t, err, tc.expectedError) {
					t.Skip()
				}

				testSameHs(t, hs, tc.hs)
			})
		}
	}()

}

func TestHSConnection_Utils(t *testing.T) {
	type testHandshakeValues struct {
		addr      string
		port      int16
		version   int16
		nextState connection.RequestType
	}

	tt := []testHandshakeValues{
		{
			addr:      "infrared",
			port:      25565,
			version:   754,
			nextState: connection.StatusRequest,
		},
		{
			addr:      "infrared-2",
			port:      25566,
			version:   755,
			nextState: connection.LoginRequest,
		},
		{
			addr:      "infrared-3",
			port:      25567,
			version:   757,
			nextState: connection.UnknownRequest,
		},
	}

	hsFactory := func(tc testHandshakeValues) connection.HSConnection {
		c1, c2 := net.Pipe()
		netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}
		conn := connection.CreateBasicPlayerConnection2(c1, netAddr)

		hs := handshaking.ServerBoundHandshake{
			ServerAddress:   protocol.String(tc.addr),
			ServerPort:      protocol.UnsignedShort(tc.port),
			ProtocolVersion: protocol.VarInt(tc.version),
			NextState:       protocol.Byte(tc.nextState),
		}

		loginData := LoginData{
			hs: hs.Marshal(),
		}

		go func() {
			loginClient(c2, loginData)
		}()
		return conn
	}

	for _, tc := range tt {
		t.Run("Server-Addr", func(t *testing.T) {
			tConn := hsFactory(tc)
			receivedAddr := connection.ServerAddr(tConn)
			if receivedAddr != tc.addr {
				t.Errorf("Expected: '%v' Got: %v", tc.addr, receivedAddr)
			}
		})
		t.Run("Server-Port", func(t *testing.T) {
			tConn := hsFactory(tc)
			receivedPort := connection.ServerPort(tConn)
			if receivedPort != tc.port {
				t.Errorf("Expected: '%v' Got: %v", tc.port, receivedPort)
			}
		})
		t.Run("Protocol-Version", func(t *testing.T) {
			tConn := hsFactory(tc)
			receivedVersion := connection.ProtocolVersion(tConn)
			if receivedVersion != tc.version {
				t.Errorf("Expected: '%v' Got: %v", tc.version, receivedVersion)
			}
		})
		t.Run("Request-Type", func(t *testing.T) {
			tConn := hsFactory(tc)
			receivedType := connection.ParseRequestType(tConn)
			if receivedType != tc.nextState {
				t.Errorf("Expected: '%v' Got: %v", tc.nextState, receivedType)
			}
		})
	}

}

type pipeConnFactory func(net.Conn) connection.PipeConnection

func testPipeConnection(factory pipeConnFactory, t *testing.T) {
	createPipe := func() (net.Conn, net.Conn) {
		client1, client2 := net.Pipe()
		server1, server2 := net.Pipe()

		c1 := factory(client2)
		c2 := factory(server2)

		go func() {
			connection.Pipe(c1, c2)
		}()

		return client1, server1
	}

	t.Run("Can read and write a single time", func(t *testing.T) {
		writingData := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		client, server := createPipe()
		readData := make([]byte, len(writingData))

		client.Write(writingData)
		server.Read(readData)

		if !bytes.Equal(readData, writingData) {
			t.Error("Received data is different from what we expected")
			t.Logf("expected:\t%v", writingData)
			t.Logf("got:\t\t%v", readData)
		}
	})

	t.Run("Can read and write twice after eachother", func(t *testing.T) {
		writingData := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		writingData2 := []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
		client, server := createPipe()
		readData := make([]byte, len(writingData))
		readData2 := make([]byte, len(writingData2))

		client.Write(writingData)
		server.Read(readData)

		client.Write(writingData2)
		server.Read(readData2)

		if !bytes.Equal(readData, writingData) {
			t.Error("Received data is different from what we expected")
			t.Logf("expected:\t%v", writingData)
			t.Logf("got:\t\t%v", readData)
		}

		if !bytes.Equal(readData2, writingData2) {
			t.Error("Received data is different from what we expected")
			t.Logf("expected:\t%v", writingData2)
			t.Logf("got:\t\t%v", readData2)
		}
	})

}

// This test probably need to fake ip from dialer
// type remoteAddrConnFactory func(net.Conn, net.Addr) connection.RemoteAddressConnection

// type remoteAddrConnTestCase struct {
// 	name          string
// 	addr          string
// 	connAddr      string
// 	expectedError error
// }

// func testremoteAddrConnection(factory remoteAddrConnFactory, t *testing.T) {
// 	standardAddr := "192.168.0.1"
// 	tt := []remoteAddrConnTestCase{
// 		{
// 			name: "normal thing should match",
// 			addr: standardAddr,
// 			connAddr: "",
// 			expectedError: nil,
// 		},
// 	}

// 	for _, tc := range tt {
// 		t.Run(tc.name, func(t *testing.T) {
// 			conn := net.Conn{}

// 		})
// 	}
// }

type serverConnFactory func(net.Conn, net.Addr) connection.ServerConnection

type serverConnTestCase struct {
	name          string
	hs            handshaking.ServerBoundHandshake
	hsPacket      protocol.Packet
	expectedError error
}

func testServerConnection(factory serverConnFactory, t *testing.T) {
	// Were arent testing handshake here thats why there only is a valid one
	validLoginHs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25665,
		ProtocolVersion: 765,
		NextState:       2,
	}
	validHSPacket := validLoginHs.Marshal()

	tt := []serverConnTestCase{
		{
			name:          "no error run",
			hs:            validLoginHs,
			hsPacket:      validHSPacket,
			expectedError: nil,
		},
	}

	testServerConnFactory := func(tc serverConnTestCase) connection.ServerConnection {
		c1, c2 := net.Pipe()
		netAddr := &net.TCPAddr{IP: net.IP("192.168.0.1")}
		conn := factory(c1, netAddr)

		hsData, _ := tc.hsPacket.Marshal()
		requestPk := status.ServerBoundRequest{}.Marshal()
		reqestData, _ := requestPk.Marshal()

		status := status.ClientBoundResponse{}.Marshal()
		statusData, _ := status.Marshal()

		server := testMCServer{
			receivedHSPk:      hsData,
			receivedRequestPk: reqestData,
			status:            statusData,
			doPing:            false,
		}
		go func() {
			server.statusServer(c2)
		}()
		return conn
	}

	func() {
		for _, tc := range tt {
			t.Run("SendPk: "+tc.name, func(t *testing.T) {
				if errors.Is(tc.expectedError, protocol.ErrInvalidPacketID) {
					t.Skip() // Need to some either on the conn side or test side
				}
				conn := testServerConnFactory(tc)

				_, err := conn.Status(tc.hsPacket)
				if shouldStopTest(t, err, tc.expectedError) {
					t.Skip()
				}
				//Check status or it matches
			})
		}
	}()

	// func() {
	// 	tt = append(tt, serverConnTestCase{
	// 		name:          "invalid packet ID",
	// 		hsPacket:      invalidLoginHs,
	// 		expectedError: protocol.ErrInvalidPacketID,
	// 	})
	// 	for _, tc := range tt {
	// 		t.Run("SendPk: "+tc.name, func(t *testing.T) {
	// 			conn := testServerConnFactory(tc)
	// 			err := conn.SendPK()
	// 			if shouldStopTest(t, err, tc.expectedError) {
	// 				t.Skip()
	// 			}

	// 			testSameHs(t, hs, tc.hs)
	// 		})
	// 	}
	// }()

}
