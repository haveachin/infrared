package gateway_test

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
)

var (
	testOuterListenerPort = 10024

	ErrStopListenerError = errors.New("use this error to stop the listener")
	ErrNoMoreConnections = errors.New("no more connections")
)

type outerListenFunc func(addr string) gateway.OuterListener

type faultyTestOutLis struct {
}

func (l *faultyTestOutLis) Start() error {
	return gateway.ErrCantStartOutListener
}

func (l *faultyTestOutLis) Accept() (connection.Connection, net.Addr) {
	return nil, nil
}

type testOutLis struct {
	conn connection.Connection

	count int
}

func (l *testOutLis) Start() error {
	return nil
}

func (l *testOutLis) Accept() (connection.Connection, net.Addr) {
	if l.count > 0 {
		// To block the thread while kinda acting like a real listener
		//  aka not returning a error bc there are not more connections
		time.Sleep(10 * time.Second)
	}
	l.count++
	return l.conn, nil
}

// Help methods
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

// type testGateway struct {
// 	handleCount               int
// 	returnErrorAfterCallCount int
// }

// func (gw *testGateway) HandleConnection(conn connection.HSConnection) error {
// 	gw.handleCount++
// 	if gw.handleCount >= gw.returnErrorAfterCallCount {
// 		return ErrStopListenerError
// 	}
// 	return nil
// }

// Actual test methods

// OuterListener Tests
func TestBasicOuterListener(t *testing.T) {
	listenerCreateFn := func(addr string) gateway.OuterListener {
		return gateway.CreateBasicOuterListener(addr)
	}

	testOuterListener(t, listenerCreateFn)
}

func testOuterListener(t *testing.T, fn outerListenFunc) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	addr := fmt.Sprintf(":%d", testOuterListenerPort)
	testOuterListenerPort++
	testPk := protocol.Packet{ID: testUnboundID}
	outerListener := fn(addr)

	go func(t *testing.T, wg *sync.WaitGroup) {
		wg.Wait()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Errorf("error was throw: %v", err)
		}
		bConn := connection.CreateBasicConnection(conn)
		err = bConn.WritePacket(testPk)
		if err != nil {
			t.Logf("got an error while writing the packet: %v", err)
		}
	}(t, &wg)

	err := outerListener.Start()
	if err != nil {
		t.Errorf("Got an error while i shouldn't. Error: %v", err)
		t.FailNow()
	}
	wg.Done()
	conn, _ := outerListener.Accept()
	receivedPk, _ := conn.ReadPacket()

	testSamePK(t, testPk, receivedPk)
}

// Infrared Listener Tests
func TestBasicListener(t *testing.T) {
	// Need changes
	tt := []struct {
		name         string
		returnsError bool
	}{
		{
			name:         "Test where start doesnt return error",
			returnsError: false,
		},
		{
			name:         "Test where start return error",
			returnsError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var outerListener gateway.OuterListener
			hsPk := protocol.Packet{ID: testLoginHSID}
			inConn := &testInConn{hsPk: hsPk}
			outerListener = &testOutLis{conn: inConn}
			if tc.returnsError {
				outerListener = &faultyTestOutLis{}
			}
			connCh := make(chan connection.HSConnection)
			l := gateway.BasicListener{OutListener: outerListener, ConnCh: connCh}

			errChannel := make(chan error)

			go func() {
				err := l.Listen()
				if err != nil {
					errChannel <- err
				}
			}()

			select {
			case err := <-errChannel:
				if !errors.Is(err, gateway.ErrCantStartListener) {
					t.Logf("unexpected error was thrown: %v", err)
					t.FailNow()
				}
			case <-time.After(1 * time.Millisecond): // err should be returned almost immediately
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			case conn := <-connCh:
				receivePk, _ := conn.ReadPacket()
				testSamePK(t, hsPk, receivePk)
			}

		})
	}
}

// type parralelServer struct {
// 	connWg        *sync.WaitGroup
// 	testWg        *sync.WaitGroup
// 	receivedConns []connection.HSConnection

// 	blockLogin  bool
// 	blockStatus bool
// }

// func (s *parralelServer) Login(conn connection.LoginConnection) error {
// 	if s.blockLogin {
// 		s.connWg.Wait()
// 	}
// 	s.receivedConns = append(s.receivedConns, conn)
// 	if !s.blockLogin {
// 		s.connWg.Done()
// 	}
// 	s.testWg.Done()
// 	return nil
// }
// func (s *parralelServer) Status(conn connection.StatusConnection) protocol.Packet {
// 	if s.blockStatus {
// 		s.connWg.Wait()
// 	}
// 	s.receivedConns = append(s.receivedConns, conn)
// 	if !s.blockStatus {
// 		s.connWg.Done()
// 	}
// 	s.testWg.Done()
// 	return protocol.Packet{}
// }

// type parralelOuterListener struct {
// 	conns     []connection.Connection
// 	connCount int

// 	wg                  *sync.WaitGroup
// 	doneWhenNoMoreConns bool
// }

// func (s *parralelOuterListener) Start() error {
// 	return nil
// }

// func (s *parralelOuterListener) Accept() connection.Connection {
// 	if s.connCount == len(s.conns) {
// 		if s.doneWhenNoMoreConns {
// 			s.wg.Done()
// 		}
// 		// To block the thread while kinda acting like a real listener
// 		//  aka not returning a error bc there are not more connections
// 		time.Sleep(10 * time.Second)
// 	}
// 	tempConn := s.conns[s.connCount]
// 	s.connCount++
// 	return tempConn
// }

// I want to leave it open to make it possible in either Listener or the gateway
//  to process multiple connection at the same time thats why I dont wanna mock
//  neither of them in here
// func TestParallelListening(t *testing.T) {

// 	// Need changes, this cant test or it also support processing multiple
// 	//  of the same connections types
// 	connWg := sync.WaitGroup{}
// 	connWg.Add(1)

// 	testWg := sync.WaitGroup{}

// 	loginPk := login.ServerLoginStart{}.Marshal()
// 	hs1 := handshaking.ServerBoundHandshake{
// 		ServerAddress: "infrared-1",
// 		NextState:     1,
// 	}
// 	blockStatus := true
// 	hs2 := handshaking.ServerBoundHandshake{
// 		ServerAddress: "infrared-2",
// 		NextState:     2,
// 	}
// 	blockLogin := false

// 	inConn1 := &testInConn{hs: hs1, loginPK: loginPk}
// 	inConn2 := &testInConn{hs: hs2, loginPK: loginPk}
// 	inConns := []connection.Connection{inConn1, inConn2}
// 	testWg.Add(len(inConns))

// 	store := &gateway.SingleServerStore{}
// 	server := &parralelServer{connWg: &connWg, testWg: &testWg, blockLogin: blockLogin, blockStatus: blockStatus}
// 	store.Server = server
// 	tGateway := gateway.CreateBasicGatewayWithStore(store, nil)

// 	outerListener := &parralelOuterListener{conns: inConns}
// 	listener := &gateway.BasicListener{Gw: &tGateway, OutListener: outerListener}

// 	go func() {
// 		listener.Listen()
// 	}()
// 	channel := make(chan struct{})
// 	go func(wg *sync.WaitGroup) {
// 		wg.Wait()
// 		channel <- struct{}{}
// 	}(&testWg)

// 	select {
// 	case <-channel:
// 		t.Log("Tasked finished before timeout")
// 	case <-time.After(100 * time.Millisecond):
// 		t.Log("Tasked timed out")
// 		t.FailNow() // Dont check other code it didnt finish anyway
// 	}

// 	if len(server.receivedConns) != len(inConns) {
// 		t.Logf("received %d connection(s) but expected %d", len(server.receivedConns), len(inConns))
// 		t.FailNow()
// 	}

// 	if server.receivedConns[0] != inConn2 {
// 		t.Error("expected conn2 to be received as first but didnt happen")
// 	}
// 	if server.receivedConns[1] != inConn1 {
// 		t.Error("didnt receive conn1 as second connection --which was expected it to be--")
// 	}

// }
