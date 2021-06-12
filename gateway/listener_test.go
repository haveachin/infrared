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

type outerListenFunc func(addr string) gateway.OuterListener

type faultyTestOutLis struct {
}

func (l *faultyTestOutLis) Start() error {
	return gateway.ErrCantStartOutListener
}

func (l *faultyTestOutLis) Accept() (net.Conn, net.Addr) {
	return nil, nil
}

type testOutLis struct {
	conn net.Conn

	count int
}

func (l *testOutLis) Start() error {
	return nil
}

func (l *testOutLis) Accept() (net.Conn, net.Addr) {
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

// Actual test methods

// OuterListener Tests
func TestBasicOuterListener(t *testing.T) {
	listenerCreateFn := func(addr string) gateway.OuterListener {
		return gateway.NewBasicOuterListener(addr)
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
		bConn := connection.NewHandshakeConn(conn, nil)
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
	cNet, _ := outerListener.Accept()
	conn := connection.NewHandshakeConn(cNet, nil)
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
			c1, c2 := net.Pipe()
			loginData := LoginData{
				hs: hsPk,
			}

			go func() {
				loginClient(c2, loginData)
			}()

			outerListener = &testOutLis{conn: c1}
			if tc.returnsError {
				outerListener = &faultyTestOutLis{}
			}
			connCh := make(chan connection.HandshakeConn)
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
			case <-time.After(defaultChanTimeout): // err should be returned almost immediately
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			case conn := <-connCh:
				receivePk, _ := conn.ReadPacket()
				testSamePK(t, hsPk, receivePk)
			}

		})
	}
}
