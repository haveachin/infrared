package gateway_test

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	testLoginHSID  byte = 5
	testLoginID    byte = 6
	testStatusHSID byte = 10

	testUnboundID byte = 31

	ErrNotImplemented = errors.New("not implemented")
	ErrNoReadLeft     = errors.New("no packets left to read")

	defaultChanTimeout = 50 * time.Millisecond
)

type testStructWithID interface {
	ID() string
}

type testServer struct {
	id           string
	loginCalled  int
	statusCalled int
}

func (s *testServer) Status(conn connection.StatusConn) protocol.Packet {
	s.statusCalled++
	return protocol.Packet{}
}

func (s *testServer) Login(conn connection.LoginConn) error {
	s.loginCalled++
	return nil
}

func (s *testServer) ID() string {
	return s.id
}

// INcomming CONNection, not obvious? Change it!
type testInConn struct {
	id int

	writeCount int
	readCount  int

	hs      handshaking.ServerBoundHandshake
	hsPk    protocol.Packet
	loginPK protocol.Packet
}

func (c *testInConn) WritePacket(p protocol.Packet) error {
	c.writeCount++
	return nil
}

func (c *testInConn) ReadPacket() (protocol.Packet, error) {
	c.readCount++
	switch c.readCount {
	case 1:
		return c.hsPk, nil
	case 2:
		return c.loginPK, nil
	default:
		return protocol.Packet{}, nil
	}

}

func (c *testInConn) ServerAddr() string {
	return string(c.hs.ServerAddress)
}

func (c *testInConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (c *testInConn) Name() (string, error) {
	return "", ErrNotImplemented
}

func (c *testInConn) HandshakePacket() (protocol.Packet, error) {
	return c.hsPk, nil
}

func (c *testInConn) Hs() (handshaking.ServerBoundHandshake, error) {
	return c.hs, nil // Always returning hs so we can really test the code or it depends on the boolean return
}

func (c *testInConn) LoginStart() (protocol.Packet, error) {
	return protocol.Packet{}, ErrNotImplemented
}

func (c *testInConn) SendStatus(status protocol.Packet) error {
	return ErrNotImplemented
}

func (c *testInConn) read(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

func (c *testInConn) write(b []byte) (n int, err error) {
	return 0, ErrNotImplemented
}

type GatewayRunner func(gwCh <-chan connection.HandshakeConn) <-chan connection.HandshakeConn

// Actual test functions
func TestFindMatchingServer_SingleServerStore(t *testing.T) {
	serverAddr := "infrared-1"

	gatewayRunner := func(gwCh <-chan connection.HandshakeConn) <-chan connection.HandshakeConn {
		connCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: connCh}
		serverStore := &gateway.SingleServerStore{Server: serverData}

		gw := gateway.NewBasicGatewayWithStore(serverStore, gwCh)
		go func() {
			gw.Start()
		}()
		return connCh
	}

	data := findServerData{
		runGateway: gatewayRunner,
		addr:       serverAddr,
		hsDepended: false,
	}

	testFindServer(data, t)
}

func TestFindServer_DefaultServerStore(t *testing.T) {
	serverAddr := "addr-1"

	gatewayRunner := func(gwCh <-chan connection.HandshakeConn) <-chan connection.HandshakeConn {
		serverStore := &gateway.DefaultServerStore{}
		for id := 2; id < 10; id++ {
			serverAddr := fmt.Sprintf("addr-%d", id)
			serverData := gateway.ServerData{ConnCh: make(chan connection.HandshakeConn)}
			serverStore.AddServer(serverAddr, serverData)
		}
		connCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: connCh}
		serverStore.AddServer(serverAddr, serverData)

		gw := gateway.NewBasicGatewayWithStore(serverStore, gwCh)
		go func() {
			gw.Start()
		}()
		return connCh
	}

	data := findServerData{
		runGateway: gatewayRunner,
		addr:       serverAddr,
		hsDepended: true,
	}

	testFindServer(data, t)
}

type findServerData struct {
	runGateway GatewayRunner
	addr       string
	hsDepended bool
}

func testFindServer(data findServerData, t *testing.T) {
	unfindableServerAddr := "pls dont use this string as actual server addr"

	type testCase struct {
		withHS     bool
		shouldFind bool
	}
	tt := []testCase{
		{
			withHS:     true,
			shouldFind: true,
		},
	}
	if data.hsDepended {
		tt1 := testCase{withHS: true, shouldFind: false}
		tt2 := testCase{withHS: false, shouldFind: false}
		tt = append(tt, tt1, tt2)
	} else {
		tt1 := testCase{withHS: false, shouldFind: true}
		tt = append(tt, tt1)
	}

	for _, tc := range tt {
		name := fmt.Sprintf("With hs: %t & shouldFind: %t ", tc.withHS, tc.shouldFind)
		t.Run(name, func(t *testing.T) {
			serverAddr := protocol.String(data.addr)
			if !tc.shouldFind {
				serverAddr = protocol.String(unfindableServerAddr)
			}
			t.Log(serverAddr)
			hs := handshaking.ServerBoundHandshake{ServerAddress: serverAddr}
			c1, c2 := net.Pipe()
			addr := &net.IPAddr{IP: []byte{1, 1, 1, 1}}
			hsConn := connection.NewBasicPlayerConn(c1, addr)
			go func() {
				pk := hs.Marshal()
				bytes, _ := pk.Marshal()
				c2.Write(bytes)
			}()

			gwCh := make(chan connection.HandshakeConn)
			serverCh := data.runGateway(gwCh)

			select {
			case <-time.After(defaultChanTimeout): //Be fast or fail >:)
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			case gwCh <- hsConn:
				t.Log("Gateway took connection")
			}

			select {
			case <-time.After(defaultChanTimeout): //Be fast or fail >:)
				if tc.shouldFind {
					t.Log("Tasked timed out")
					t.FailNow() // Dont check other code it didnt finish anyway
				}
			case <-serverCh:
				t.Log("Server returned connection")
				// Maybe validate here or it received the right connection?
			}

		})
	}

}

// // This test is meant for testing how it all works together
// //  so only the INcomming and OUTgoing connection should be mocked
// func TestInToOutBoundry(t *testing.T) {

// 	wg := sync.WaitGroup{}
// 	wg.Add(2)
// 	channel := make(chan struct{})
// 	go func() {
// 		wg.Wait()
// 		channel <- struct{}{}
// 	}()

// 	serverAddr := "infrared.test"
// 	HsPk := handshaking.ServerBoundHandshake{
// 		ServerAddress:   protocol.String(serverAddr),
// 		ServerPort:      25565,
// 		ProtocolVersion: 754,
// 		NextState:       2,
// 	}.Marshal()

// 	loginPk := login.ServerLoginStart{Name: "infrared"}.Marshal()

// 	firstPipePk := protocol.Packet{ID: 25, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}}

// 	inConnWritePackets := []protocol.Packet{HsPk, loginPk, firstPipePk}

// 	tOutConn := &bTestConnection{wg: &wg}
// 	tInConn := &bTestConnection{wg: &wg, pks: inConnWritePackets}

// 	// Setup server stuff
// 	serverConnFactory := func() connection.ServerConnection {
// 		return connection.CreateBasicServerConn(tOutConn, protocol.Packet{})
// 	}

// 	mcServer := &server.MCServer{
// 		ConnFactory: serverConnFactory,
// 	}
// 	store := &gateway.DefaultServerStore{}
// 	store.AddServer(serverAddr, mcServer)

// 	tGateway := gateway.CreateBasicGatewayWithStore(store, nil)

// 	ipAddr := &net.TCPAddr{IP: []byte{101, 12, 23, 85}, Port: 50674}
// 	playerConn := connection.CreateBasicPlayerConnection(tInConn, ipAddr)
// 	outerListener := &testOutLis{conn: playerConn}

// 	listener := &gateway.BasicListener{Gw: &tGateway, OutListener: outerListener}

// 	// Start Testing stuff
// 	go func() {
// 		listener.Listen()
// 	}()

// 	timeout := time.After(100 * time.Millisecond)
// 	select {
// 	case <-channel:
// 		t.Log("Tasked finished before timeout")
// 	case <-timeout:
// 		t.Log("Tasked timed out")
// 		t.FailNow() // Dont check other code it didnt finish anyway
// 	}

// 	if !(tInConn.readCount == len(inConnWritePackets)) {
// 		t.Errorf("Read was only called %d times instead of the expected %d", tInConn.readCount, len(inConnWritePackets))
// 	}

// }

// // Boundry test struct
// type bTestConnection struct {
// 	//implements interface ServerConnection atm, might change later
// 	writeCount int
// 	readCount  int

// 	pks         []protocol.Packet
// 	receivedPks []protocol.Packet

// 	wg         *sync.WaitGroup
// 	markedDone bool
// }

// func (c *bTestConnection) WritePacket(p protocol.Packet) error {
// 	if c.receivedPks == nil {
// 		c.receivedPks = make([]protocol.Packet, 1)
// 	}
// 	c.receivedPks = append(c.receivedPks, p)
// 	c.writeCount++
// 	return nil
// }

// func (c *bTestConnection) ReadPacket() (protocol.Packet, error) {
// 	if c.readCount == len(c.pks) {
// 		if !c.markedDone {
// 			c.wg.Done()
// 			c.markedDone = true
// 		}
// 		return protocol.Packet{}, ErrNoReadLeft
// 	}
// 	pkToReturn := c.pks[c.readCount]
// 	c.readCount++
// 	return pkToReturn, nil
// }

// func (c *bTestConnection) read(b []byte) (n int, err error) {
// 	if c.readCount == len(c.pks) {
// 		if !c.markedDone {
// 			c.wg.Done()
// 			c.markedDone = true
// 		}
// 		return 0, ErrNoReadLeft
// 	}
// 	p := c.pks[c.readCount]
// 	c.readCount++

// 	pk, _ := p.Marshal()

// 	for i := 0; i < len(pk); i++ {
// 		b[i] = pk[i]
// 	}
// 	return len(pk), nil
// }

// func (c *bTestConnection) write(b []byte) (n int, err error) {
// 	pk := protocol.Packet{
// 		ID:   b[1],
// 		Data: b[2:],
// 	}
// 	c.receivedPks = append(c.receivedPks, pk)
// 	c.writeCount++
// 	return 0, nil
// }
