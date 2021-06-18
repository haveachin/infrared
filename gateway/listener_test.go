package gateway_test

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
)

var (
	ErrStopListenerError = &net.OpError{Op: "accept", Err: errors.New("no more connections")}
	ErrNoConnections     = errors.New("no more connections")
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

type testListener struct {
	conn              net.Conn
	returnNewConn     bool
	returnError       bool
	returnClosedError bool
	count             int
}

func (l *testListener) Close() error {
	return nil
}

func (l *testListener) AllowNewConn() {
	l.returnNewConn = true
}

func (l *testListener) Addr() net.Addr {
	return nil
}

func (l *testListener) Accept() (net.Conn, error) {
	if l.count > 0 {
		for {
			if l.returnNewConn {
				l.returnNewConn = false
				break
			} else {
				// Just wait for some time
				<-time.After(2*time.Millisecond)
			}
		}
	}
	l.count++
	var err error
	if l.returnError {
		err = ErrNoConnections
	}
	if l.returnClosedError {
		err = ErrStopListenerError
	}
	return l.conn, err
}

func (l *testListener) createNewConn() net.Conn {
	c1, c2 := net.Pipe()
	l.conn = c1
	return c2
}

// Actual test method(s)
func TestBasicListener(t *testing.T) {
	hsPk := protocol.Packet{ID: testLoginHSID}
	loginData := LoginData{
		hs: hsPk,
	}
	checkConnection := func(t *testing.T, conn connection.HandshakeConn) {
		expectedPk := hsPk
		receivePk, _ := conn.ReadPacket()
		sameID := expectedPk.ID == receivePk.ID
		sameData := bytes.Equal(expectedPk.Data, receivePk.Data)
		if !sameID && sameData {
			t.Helper()
			t.Logf("expected:\t%v", expectedPk)
			t.Logf("got:\t%v", receivePk)
			t.Error("Received packet is different from what we expected")
		}
	}

	tt := []struct {
		name                   string
		returnError            bool
		expectedError          error
		shouldContinueToListen bool
		returnCloseError       bool
	}{
		{
			name:                   "Test where listening doesnt have an error",
			returnError:            false,
			shouldContinueToListen: true,
		},
		{
			name:                   "Test where listening doesnt have an error",
			returnError:            false,
			shouldContinueToListen: true,
		},
		{
			name:                   "Test where listening does have an error",
			returnError:            true,
			expectedError:          ErrNoConnections,
			shouldContinueToListen: true,
		},
		{
			name:                   "Test where listener closes",
			returnError:            true,
			expectedError:          ErrStopListenerError,
			shouldContinueToListen: false,
			returnCloseError:       true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			errChannel := make(chan error)
			errLogger := func(err error) {
				t.Log(err)
				errChannel <- err
			}
			listener := &testListener{
				returnError:       tc.returnError,
				returnClosedError: tc.returnCloseError,
			}

			conn := listener.createNewConn()
			go func() {
				loginClient(conn, loginData)
			}()
			connCh := make(chan connection.HandshakeConn)
			l := gateway.NewBasicListener(listener, connCh, errLogger)

			go func() {
				l.Listen()
			}()

			select {
			case err := <-errChannel:
				if !errors.Is(err, tc.expectedError) {
					t.Logf("unexpected error was thrown: %v", err)
					t.FailNow()
				}
			case <-time.After(defaultChanTimeout):
				t.Log("Tasked timed out")
				t.FailNow()
			case conn := <-connCh:
				if tc.returnError {
					t.Logf("No error was thrown while it should have")
					t.FailNow()
				}
				checkConnection(t, conn)
			}

			listener.returnError = false
			listener.returnClosedError = false
			conn = listener.createNewConn()
			listener.returnNewConn = true

			select {
			case err := <-errChannel:
				t.Logf("unexpected error was thrown: %v", err)
				t.FailNow()
			case <-time.After(defaultChanTimeout):
				if tc.shouldContinueToListen {
					t.Log("Tasked timed out")
					t.FailNow()
				}
			case <-connCh:
				if !tc.shouldContinueToListen {
					t.Log("Listener should have stopped")
					t.FailNow()
				}
			}

		})
	}
}
