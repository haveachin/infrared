package gateway_test

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
)

var (
	outerListenerPort int = 10024
)

type outerListenFunc func(addr string) gateway.OuterListener

type faultyTestOutLis struct {
}

func (l *faultyTestOutLis) Start() error {
	return gateway.ErrCantStartOutListener
}

func (l *faultyTestOutLis) Accept() connection.Connection {
	return nil
}

type testOutLis struct {
	conn connection.Connection
}

func (l *testOutLis) Start() error {
	return nil
}

func (l *testOutLis) Accept() connection.Connection {
	return l.conn
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

type testGateway struct {
	handleCount int
}

func (gw *testGateway) HandleConnection(conn connection.PlayerConnection) {
	gw.handleCount++
}

// Actual test methods

// OuterListener Tests
func TestBasicOuterListener(t *testing.T) {
	listenerCreateFn := func(addr string) gateway.OuterListener {
		return gateway.CreateBasicOuterListener(addr)
	}

	testOuterListener(t, listenerCreateFn)
}

//This is how you can easily test a second outerlistener implemenation
func TestBasicOuterListener2(t *testing.T) {
	listenerCreateFn := func(addr string) gateway.OuterListener {
		return gateway.CreateBasicOuterListener(addr)
	}

	testOuterListener(t, listenerCreateFn)
}

func testOuterListener(t *testing.T, fn outerListenFunc) {
	// TODO: This is a race condition, need to implement a better way
	addr := fmt.Sprintf(":%d", outerListenerPort)
	outerListenerPort++
	testPk := protocol.Packet{ID: testUnboundID}
	outerListener := fn(addr)

	go func(t *testing.T) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Errorf("error was throw: %v", err)
		}
		bConn := connection.CreateBasicConnection(conn)
		bConn.WritePacket(testPk)
	}(t)

	err := outerListener.Start()
	if err != nil {
		t.Errorf("Got an error while i shouldn't. Error: %v", err)
		t.FailNow()
	}
	conn := outerListener.Accept()
	receivedPk, _ := conn.ReadPacket()

	testSamePK(t, testPk, receivedPk)
}

// Infrared Listener Tests

func TestBasicListener(t *testing.T) {
	tt := []struct {
		name              string
		startReturnsError bool
	}{
		{
			name:              "Test where start doesnt return error",
			startReturnsError: false,
		},
		{
			name:              "Test where start return error",
			startReturnsError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var outerListener gateway.OuterListener
			hsPk := protocol.Packet{ID: testLoginHSID}
			inConn := &testInConn{hsPk: hsPk, hasHS: true}
			outerListener = &testOutLis{conn: inConn}
			if tc.startReturnsError {
				outerListener = &faultyTestOutLis{}
			}
			gw := &testGateway{}
			listener := gateway.BasicListener{OutListener: outerListener, Gw: gw}

			err := listener.Listen()

			if err != nil {
				if tc.startReturnsError {
					return
				}
				t.Logf("error was throw: %v", err)
				t.FailNow()
			}

			if gw.handleCount != 1 {
				t.Error("Listener didnt call to gateway when there was a connection ready to be received")
			}
		})
	}

}
