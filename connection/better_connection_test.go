package connection_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	defaultChanTimeout = 5 * time.Millisecond
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

func TestBasicPlayerConn(t *testing.T) {

	innerFactory := func(conn net.Conn, addr net.Addr) *connection.BasicPlayerConn {
		return connection.NewBasicPlayerConn(conn, addr)
	}

	// loginFactory := func(conn net.Conn, addr net.Addr) connection.LoginConn {
	// 	return innerFactory(conn, addr)
	// }
	// testLoginConn(loginFactory, t)

	hsFactory := func(conn net.Conn, addr net.Addr) connection.HandshakeConn {
		return innerFactory(conn, addr)
	}
	testHSConn(hsFactory, t)

	pipeFactory := func(conn net.Conn) connection.PipeConn {
		return innerFactory(conn, &net.IPAddr{})
	}
	testPipeConn(pipeFactory, t)

	connFactory := func(conn net.Conn) connection.Conn {
		return innerFactory(conn, &net.IPAddr{})
	}
	testConn(connFactory, t)
}

func TestServerConn(t *testing.T) {

	innerFactory := func(conn net.Conn) *connection.BasicServerConn {
		return connection.NewBasicServerConn(conn)
	}

	pipeFactory := func(conn net.Conn) connection.PipeConn {
		return innerFactory(conn)
	}
	testPipeConn(pipeFactory, t)

	connFactory := func(conn net.Conn) connection.Conn {
		return innerFactory(conn)
	}
	testConn(connFactory, t)

}

func TestBasicConn(t *testing.T) {
	innerFactory := func(conn net.Conn) *connection.BasicConn {
		return connection.NewBasicConn(conn)
	}

	byteFactory := func(conn net.Conn) connection.ByteConn {
		return innerFactory(conn)
	}
	testByteConn(byteFactory, t)

}

type hsConnFactory func(net.Conn, net.Addr) connection.HandshakeConn

type hsConnTestCase struct {
	name     string
	hs       handshaking.ServerBoundHandshake
	hsPacket protocol.Packet
	addr     net.Addr
}

func testHSConn(factory hsConnFactory, t *testing.T) {
	defaultAddr := &net.IPAddr{IP: []byte{127, 0, 0, 1}}
	validLoginHs := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25665,
		ProtocolVersion: 765,
		NextState:       2,
	}
	validHSPacket := validLoginHs.Marshal()
	validLoginHs2 := handshaking.ServerBoundHandshake{
		ServerAddress:   "infrared",
		ServerPort:      25665,
		ProtocolVersion: 765,
		NextState:       2,
	}
	validHSPacket2 := validLoginHs.Marshal()
	tt := []hsConnTestCase{
		{
			name:     "no error run",
			hs:       validLoginHs,
			hsPacket: validHSPacket,
			addr:     &net.IPAddr{IP: []byte{1, 1, 1, 1}},
		},
		{
			name:     "different hs run",
			hs:       validLoginHs2,
			hsPacket: validHSPacket,
		},
		{
			name:     "different packet run",
			hs:       validLoginHs,
			hsPacket: validHSPacket2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.addr == nil {
				tc.addr = defaultAddr
			}

			hsConn := factory(nil, tc.addr)
			hsConn.SetHandshake(tc.hs)
			hsConn.SetHandshakePacket(tc.hsPacket)

			pk := hsConn.HandshakePacket()
			testSamePK(t, tc.hsPacket, pk)

			hs := hsConn.Handshake()
			if hs != tc.hs {
				t.Logf("expected:\t%v", tc.hs)
				t.Logf("got:\t\t%v", hs)
				t.Error("Received different handshake from what we expected")
			}

			if hsConn.RemoteAddr().String() != tc.addr.String() {
				t.Logf("expected:\t%v", tc.addr.String())
				t.Logf("got:\t\t%v", hsConn.RemoteAddr().String())
				t.Error("Received different handshake from what we expected")
			}

		})
	}

}

func TestHSConn_Utils(t *testing.T) {
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

	hsFactory := func(tc testHandshakeValues) connection.HandshakeConn {
		conn := connection.NewBasicPlayerConn(nil, nil)

		hs := handshaking.ServerBoundHandshake{
			ServerAddress:   protocol.String(tc.addr),
			ServerPort:      protocol.UnsignedShort(tc.port),
			ProtocolVersion: protocol.VarInt(tc.version),
			NextState:       protocol.Byte(tc.nextState),
		}
		conn.SetHandshake(hs)

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

type pipeConnFactory func(net.Conn) connection.PipeConn

// Need tests when client & server close connection than it will also close the other
//  connection and continues to run the code
func testPipeConn(factory pipeConnFactory, t *testing.T) {
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

type byteConnFactory func(net.Conn) connection.ByteConn

// If one of these test times out
func testByteConn(factory byteConnFactory, t *testing.T) {
	dataBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

	conns := func() (connection.ByteConn, connection.ByteConn) {
		c1, c2 := net.Pipe()
		client := factory(c1)
		server := factory(c2)
		return client, server
	}

	checkNoErr := func(err error, t *testing.T) {
		if err != nil {
			t.Error("didnt expect an error but got one")
			t.Log(err)
		}
	}

	checkIOLength := func(n int, t *testing.T) {
		if n != len(dataBytes) {
			t.Error("Received different lengths of data")
			t.Logf("expected:\t%v", len(dataBytes))
			t.Logf("got:\t\t%v", n)
		}
	}

	checkEOFErr := func(err error, t *testing.T) {
		if !errors.Is(io.EOF, err) {
			t.Error("expect an EOF error but didnt got one")
			t.Log(err)
		}
	}

	t.Run("Can write", func(t *testing.T) {
		client, server := conns()
		go func() {
			readBytes := make([]byte, len(dataBytes))
			server.Read(readBytes)
		}()

		n, err := client.Write(dataBytes)
		checkNoErr(err, t)
		checkIOLength(n, t)
	})

	t.Run("Can read", func(t *testing.T) {
		client, server := conns()
		go func() {
			client.Write(dataBytes)
		}()
		readBytes := make([]byte, len(dataBytes))
		n, err := server.Read(readBytes)
		checkNoErr(err, t)
		checkIOLength(n, t)

		if !bytes.Equal(dataBytes, readBytes) {
			t.Error("Received data is different from what we expected")
			t.Logf("expected:\t%v", dataBytes)
			t.Logf("got:\t\t%v", readBytes)
		}
	})

	t.Run("Can close", func(t *testing.T) {
		readBytes := make([]byte, len(dataBytes))
		client, server := conns()
		go func() {
			client.Close()
			// Writing data to prevent timeout if close doesnt work
			client.Write(dataBytes)
		}()
		_, err := server.Read(readBytes)
		checkEOFErr(err, t)
	})

}

type connFactory func(net.Conn) connection.Conn

type connTestCase struct {
	name        string
	pk          protocol.Packet
	expecterErr error
}

func testConn(factory connFactory, t *testing.T) {
	normalPacket := protocol.Packet{ID: 0x15, Data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}

	tt := []connTestCase{
		{
			name:        "no error run",
			pk:          normalPacket,
			expecterErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run("", func(t *testing.T) {
			c1, c2 := net.Pipe()
			conn1 := factory(c1)
			conn2 := factory(c2)

			go func() {
				conn1.WritePacket(tc.pk)
			}()

			pk, err := conn2.ReadPacket()
			if shouldStopTest(t, err, tc.expecterErr) {
				t.FailNow()
			}

			testSamePK(t, tc.pk, pk)
		})
	}

	conns := func() (connection.Conn, connection.Conn) {
		c1, c2 := net.Pipe()
		client := factory(c1)
		server := factory(c2)
		return client, server
	}

	checkNoErr := func(err error, t *testing.T) {
		if err != nil {
			t.Error("didnt expect an error but got one")
			t.Log(err)
		}
	}

	t.Run("Can write", func(t *testing.T) {
		conn1, conn2 := conns()
		go func() {
			conn1.WritePacket(normalPacket)
		}()

		pk, err := conn2.ReadPacket()
		checkNoErr(err, t)
		testSamePK(t, normalPacket, pk)
	})

	t.Run("Can read", func(t *testing.T) {
		conn1, conn2 := conns()
		go func() {
			conn2.ReadPacket()
		}()

		err := conn1.WritePacket(normalPacket)
		checkNoErr(err, t)
	})

}
