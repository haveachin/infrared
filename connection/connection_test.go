package connection_test

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
	"github.com/haveachin/infrared/protocol/login"
	"github.com/haveachin/infrared/protocol/status"
)

type loginData struct {
	hs         handshaking.ServerBoundHandshake
	loginStart login.ServerLoginStart
}

func loginClient(conn net.Conn, data loginData) {
	// hs := handshaking.ServerBoundHandshake{
	// 	ServerAddress:   "infrared",
	// 	ServerPort:      25665,
	// 	ProtocolVersion: 765,
	// 	NextState:       2,
	// }
	// loginStart := login.ServerLoginStart{
	// 	Name: "Drago",
	// }
	hsPk := data.hs.Marshal()
	loginPk := data.loginStart.Marshal()

	bytes, _ := hsPk.Marshal()
	conn.Write(bytes)

	bytes, _ = loginPk.Marshal()
	conn.Write(bytes)
}

type statusData struct {
	withPing bool
	hs       handshaking.ServerBoundHandshake
	status   status.ServerBoundRequest
}

func statusWithPingClient(conn net.Conn, data statusData) {
	// hs := handshaking.ServerBoundHandshake{
	// 	ServerAddress:   "infrared",
	// 	ServerPort:      25665,
	// 	ProtocolVersion: 765,
	// 	NextState:       1,
	// }
	hsPk := data.hs.Marshal()

	requestPk := data.status.Marshal()

	bytes, _ := hsPk.Marshal()
	conn.Write(bytes)

	bytes, _ = requestPk.Marshal()
	conn.Write(bytes)

	responseData := make([]byte, 0xffff)
	conn.Read(responseData)

	if data.withPing {
		pingData := make([]byte, 2)
		conn.Read(pingData)

		conn.Write(pingData)
	}

}

var (
	testLoginHSID  byte = 5
	testLoginID    byte = 6
	testStatusHSID byte = 10

	testUnboundID byte = 31

	testSendID    byte = 41
	testReceiveID byte = 42

	ErrReadPacket      = errors.New("cant read packet bc of test")
	ErrNotImplemented  = errors.New("not implemented")
	ErrOnPurposeReturn = errors.New("this error is returned on purpose")
)

// Should implement connection.Connect
type testConnection struct {
	hs         protocol.Packet
	loginPK    protocol.Packet
	readCount  int
	writeCount int

	returnErrors bool
}

func (c *testConnection) conn() net.Conn {
	return nil
}

func (c *testConnection) WritePacket(p protocol.Packet) error {
	c.writeCount++
	return nil
}

func (c *testConnection) ReadPacket() (protocol.Packet, error) {
	c.readCount++

	var err error = nil
	if c.returnErrors {
		err = ErrReadPacket
	}

	switch c.readCount {
	case 1:
		return c.hs, err
	case 2:
		return c.loginPK, err
	default:
		return protocol.Packet{}, err
	}
}

func (c *testConnection) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *testConnection) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

type serverTestConn struct {
	status         protocol.Packet
	readCount      int
	writeCount     int
	receivedPacket protocol.Packet
}

func (c *serverTestConn) conn() net.Conn {
	return nil
}

func (c *serverTestConn) WritePacket(p protocol.Packet) error {
	c.writeCount++
	c.receivedPacket = p
	return nil
}

func (c *serverTestConn) ReadPacket() (protocol.Packet, error) {
	c.readCount++
	switch c.readCount {
	case 1:
		return c.status, nil
	default:
		return protocol.Packet{}, nil
	}

}

func (c *serverTestConn) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *serverTestConn) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *serverTestConn) receivedPk() protocol.Packet {
	return c.receivedPacket
}

// util functions
func checkReceivedPk(t *testing.T, fn func(protocol.Packet) (protocol.Packet, error), expectedPK protocol.Packet) {
	pk, err := fn(protocol.Packet{})
	if err != nil {
		t.Errorf("got error: %v", err)
	}

	testSamePK(t, expectedPK, pk)
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
	if !samePK(expected, received) {
		t.Logf("expected:\t%v", expected)
		t.Logf("got:\t\t%v", received)
		t.Error("Received packet is different from what we expected")
	}
}

func samePK(expected, received protocol.Packet) bool {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	return sameID && sameData
}

// Actual test methods
// func TestBasicLoginConnection(t *testing.T) {

// 	expectedHS := protocol.Packet{ID: testLoginHSID}
// 	expecedLogin := protocol.Packet{ID: testLoginID}
// 	conn := &loginTestConn{hs: expectedHS, loginPK: expecedLogin}

// 	lConn := connection.CreateBasicPlayerConnection(conn)

// 	checkReceivedPk(t, lConn.HsPk, expectedHS)
// 	checkReceivedPk(t, lConn.LoginStart, expecedLogin)
// }

func TestBasicServerConnection_Status(t *testing.T) {
	expectedStatus, _ := infrared.StatusConfig{
		VersionName:    "Latest",
		ProtocolNumber: 1,
		MaxPlayers:     999,
		MOTD:           "One of a kind server!",
	}.StatusResponsePacket()
	expectedRequest := protocol.Packet{ID: testStatusHSID}

	conn := &serverTestConn{status: expectedStatus}
	sConn := connection.CreateBasicServerConn(conn, expectedRequest)

	checkReceivedPk(t, sConn.Status, expectedStatus)

	if conn.writeCount != 1 {
		t.Error("there were more or less than 1 write requests to the mcServer")
	}

	if conn.readCount != 1 {
		t.Error("there were more or less than 1 read requests to the mcServer")
	}

	receivedReq := conn.receivedPk()

	testSamePK(t, expectedRequest, receivedReq)

}

func TestBasicServerConnection_SendPK(t *testing.T) {
	expectedRequest := protocol.Packet{ID: testUnboundID}

	conn := &serverTestConn{}
	sConn := connection.CreateBasicServerConn(conn, expectedRequest)

	err := sConn.SendPK(expectedRequest)
	if err != nil {
		t.Errorf("got unexepected error: %v", err)
	}

	receivedReq := conn.receivedPk()

	testSamePK(t, expectedRequest, receivedReq)

}

func TestBasicConnectionStuff(t *testing.T) {
	tt := []struct {
		name string
		addr net.Addr
		conn net.Conn
	}{
		{},
	}
	for _, tc := range tt {
		t.Run("", func(t *testing.T) {
			fmt.Println(tc)
		})
	}

}

func TestBasicConnection(t *testing.T) {
	c1, c2 := net.Pipe()
	conn1 := connection.CreateBasicConnection(c1)
	testData := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

	pkSend := protocol.Packet{ID: testSendID, Data: testData}
	pkReceive := protocol.Packet{ID: testReceiveID, Data: testData}

	// READING AND WRITING OF PACKET SHOULD NOT DEPEND ON OTHER PROJECT CODE NO MATTER WHAT
	// AN OVERLOOKED ERROR IN THAT COULD SNOWBALL INTO MANY PROBLEMS
	// Testing or Reading(Receiving) packets works
	go func() {
		pk, _ := pkReceive.Marshal() // We arent testing here or this method works
		c2.Write(pk)
	}()
	pk1, _ := conn1.ReadPacket() //Need testing for the error
	testSamePK(t, pkReceive, pk1)

	// Testing or Writing(Sending) packets works
	go func(t *testing.T) {
		t.Logf("sending bytes: %v", pkSend)
		_ = conn1.WritePacket(pkSend) //Need testing for the error
	}(t)

	//pkLength + pkID + pkData
	lengthIncommingBytes := 1 + 1 + len(testData)
	var data []byte = make([]byte, lengthIncommingBytes)
	c2.Read(data)

	// First Byte is packet length (simplified)
	pk2 := protocol.Packet{
		ID:   data[1],
		Data: data[2:],
	}

	testSamePK(t, pkSend, pk2)

}

func TestBasicPlayerConnection(t *testing.T) {
	connFactory := func(data hsConnData) connection.HSConnection {
		return connection.CreateBasicPlayerConnection(data.conn, data.remoteAddr)
	}

	testHSConnection_HsPk(t, connFactory)
	testHSConnection_Hs(t, connFactory)
}

type hsConnData struct {
	conn       connection.Connection
	remoteAddr net.Addr
}

type hsConnFactory func(data hsConnData) connection.HSConnection

func testHSConnection_HsPk(t *testing.T, hsConnFactory hsConnFactory) {
	hs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25565,
		ProtocolVersion: 754,
		NextState:       1,
	}
	HsPk := hs.Marshal()
	remoteAddr := &net.TCPAddr{IP: []byte{111, 0, 0, 0}, Port: 25566}
	tt := []struct {
		name            string
		pk              protocol.Packet
		connReturnError bool
		expectedError   error
	}{
		{
			name: "normal test",
			pk:   HsPk,
		},
		{
			name:            "error test run - connection will always return error",
			pk:              HsPk,
			connReturnError: true,
			expectedError:   connection.ErrCantGetHSPacket,
		},
	}

	for _, tc := range tt {
		t.Run("HsPk method: "+tc.name, func(t *testing.T) {
			tConn := &testConnection{hs: tc.pk, returnErrors: tc.connReturnError}
			data := hsConnData{conn: tConn, remoteAddr: remoteAddr}
			conn := hsConnFactory(data)

			receivedPK, err := conn.HsPk()
			if err != nil && err == tc.expectedError {
				// Do nothing
			} else if err != nil && err != tc.expectedError {
				t.Logf("expected error: %v", tc.expectedError)
				t.Logf("got error: %v", err)
				t.Error("expected an error but got a wrong one")
			} else if err == nil && tc.expectedError != nil {
				t.Logf("expected error: %v", tc.expectedError)
				t.Error("expected error but didnt get any")
			} else {
				testSamePK(t, tc.pk, receivedPK)

				if remoteAddr.String() != conn.RemoteAddr().String() {
					t.Error("remote addresses didnt match")
				}
			}

		})

	}
}

func testHSConnection_Hs(t *testing.T, hsConnFactory hsConnFactory) {
	hs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25565,
		ProtocolVersion: 754,
		NextState:       1,
	}
	HsPk := hs.Marshal()

	wrongIDHsPk := protocol.Packet{ID: 0x99, Data: []byte{0x99, 0x00}}
	remoteAddr := &net.TCPAddr{IP: []byte{111, 0, 0, 0}, Port: 25566}
	tt := []struct {
		name            string
		pk              protocol.Packet
		hs              handshaking.ServerBoundHandshake
		connReturnError bool
		expectedError   error
	}{
		{
			name: "normal test",
			pk:   HsPk,
			hs:   hs,
		},
		{
			name:            "error test run - connection will always return error",
			pk:              HsPk,
			hs:              hs,
			connReturnError: true,
			expectedError:   connection.ErrCantGetHSPacket,
		},
		{
			name:            "faulty ID HS packet",
			pk:              wrongIDHsPk,
			connReturnError: false,
			expectedError:   protocol.ErrInvalidPacketID,
		},
	}

	for _, tc := range tt {
		t.Run("Hs method: "+tc.name, func(t *testing.T) {
			tConn := &testConnection{hs: tc.pk, returnErrors: tc.connReturnError}
			data := hsConnData{conn: tConn, remoteAddr: remoteAddr}
			conn := hsConnFactory(data)

			receivedPK, err := conn.Handshake()
			if err != nil && err == tc.expectedError {
				// Do nothing
			} else if err != nil && err != tc.expectedError {
				t.Logf("expected error: %v", tc.expectedError)
				t.Logf("got error: %v", err)
				t.Error("expected an error but got a wrong one")
			} else if err == nil && tc.expectedError != nil {
				t.Logf("expected error: %v", tc.expectedError)
				t.Error("expected error but didnt get any")
			} else {
				testSameHs(t, tc.hs, receivedPK)
			}
		})
	}
}

type testHSConnection struct {
	hs handshaking.ServerBoundHandshake
}

func (c testHSConnection) WritePacket(pk protocol.Packet) error {
	return nil
}

func (c testHSConnection) ReadPacket() (protocol.Packet, error) {
	return protocol.Packet{}, nil
}

func (c testHSConnection) conn() net.Conn {
	return nil
}

func (c testHSConnection) Handshake() (handshaking.ServerBoundHandshake, error) {
	return c.hs, nil
}

func (c testHSConnection) HsPk() (protocol.Packet, error) {
	return protocol.Packet{}, nil
}

func (c testHSConnection) RemoteAddr() net.Addr {
	return nil
}

func (c testHSConnection) Read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c testHSConnection) Write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func TestHSConnection_Utils(t *testing.T) {
	tt := []struct {
		addr      string
		port      int16
		version   int16
		nextState connection.RequestType
	}{
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

	for _, tc := range tt {
		name := fmt.Sprintf("Handshak run: %s", tc.addr)
		t.Run(name, func(t *testing.T) {
			hs := handshaking.ServerBoundHandshake{
				ServerAddress:   protocol.String(tc.addr),
				ServerPort:      protocol.UnsignedShort(tc.port),
				ProtocolVersion: protocol.VarInt(tc.version),
				NextState:       protocol.Byte(tc.nextState),
			}
			tConn := &testHSConnection{hs: hs}

			receivedAddr := connection.ServerAddr(tConn)
			if receivedAddr != tc.addr {
				t.Errorf("Expected: '%v' Got: %v", tc.addr, receivedAddr)
			}

			receivedPort := connection.ServerPort(tConn)
			if receivedPort != tc.port {
				t.Errorf("Expected: '%v' Got: %v", tc.port, receivedPort)
			}

			receivedVersion := connection.ProtocolVersion(tConn)
			if receivedVersion != tc.version {
				t.Errorf("Expected: '%v' Got: %v", tc.version, receivedVersion)
			}

			receivedType := connection.ParseRequestType(tConn)
			if receivedType != tc.nextState {
				t.Errorf("Expected: '%v' Got: %v", tc.nextState, receivedType)
			}
		})
	}

}

func TestPipe(t *testing.T) {
	c1Data := [][]byte{{1}, {1}, {1}, {1}, {1}, {1}}
	c2Data := [][]byte{{2}, {2}, {2}, {2}, {2}, {2}}
	emptyData := [][]byte{}

	wg := sync.WaitGroup{}
	c1 := &testPipeConn{wg: &wg, sendData: c1Data, receivedData: emptyData}
	c2 := &testPipeConn{wg: &wg, sendData: c2Data, receivedData: emptyData}
	wg.Add(2)
	channel := make(chan struct{})
	go func() {
		wg.Wait()
		channel <- struct{}{}
	}()

	connection.Pipe(c1, c2)

	timeout := time.After(1 * time.Second)
	select {
	case <-channel:
		t.Log("Tasked finished before timeout")
	case <-timeout:
		t.Log("Tasked timed out")
		t.FailNow() // Dont check other code it didnt finish anyway
	}

	if len(c1.receivedData) != len(c2.sendData) {
		t.Error("c1 a different write data size than c2's read data (data got lost or too much data?)")
	}
	if len(c2.receivedData) != len(c1.sendData) {
		t.Error("c2 a different write data size than c1's read data (data got lost or too much data?)")
	}

	equalFn := func(s1, s2 [][]byte, t *testing.T) bool {
		for i := 0; i < len(s1); i++ {
			if !bytes.Equal(s1[i], s2[i]) {
				return false
			}
		}
		return true
	}

	if !equalFn(c1.sendData, c2.receivedData, t) {
		t.Errorf("expected:\t%v", c1.sendData)
		t.Errorf("got:\t%v", c2.receivedData)
		t.Error("Data doesnt match")
	}

	if !equalFn(c2.sendData, c1.receivedData, t) {
		t.Errorf("expected:\t%v", c1.sendData)
		t.Errorf("got:\t%v", c2.receivedData)
		t.Error("Data doesnt match")
	}

}

var ErrNoMoreDataToRead = errors.New("no more data to read in this pipe connection")

type testPipeConn struct {
	receivedData [][]byte
	sendData     [][]byte

	sendCount     int
	receivedCount int

	rShouldReturnErr bool
	wShouldReturnErr bool

	wg *sync.WaitGroup
}

func (c *testPipeConn) Read(b []byte) (n int, err error) {
	if c.rShouldReturnErr {
		return 0, ErrOnPurposeReturn
	}
	if len(c.sendData) == c.sendCount {
		return 0, ErrNoMoreDataToRead
	}
	data := c.sendData[c.sendCount]

	for i := 0; i < len(data); i++ {
		b[i] = data[i]
	}
	c.sendCount++

	if c.sendCount == len(c.sendData) {
		c.wg.Done()
	}
	return len(data), nil
}

func (c *testPipeConn) Write(b []byte) (n int, err error) {
	if c.wShouldReturnErr {
		return 0, ErrOnPurposeReturn
	}
	if c.receivedData == nil {
		c.receivedData = make([][]byte, 0)
	}
	c.receivedData = append(c.receivedData, b)
	c.receivedCount++
	return len(b), nil
}

func (c *testPipeConn) WritePacket(p protocol.Packet) error {
	return nil
}

func (c *testPipeConn) ReadPacket() (protocol.Packet, error) {
	return protocol.Packet{}, ErrNotImplemented
}
