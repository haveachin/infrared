package gateway_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
)

var (
	ErrNoConnections        = errors.New("no more connections")
	ErrStopListenerError    = &net.OpError{Op: "accept", Err: ErrNoConnections}
	ErrTimeoutListenerError = &net.OpError{Op: "accept", Err: &testTimeoutError{}}
)

type testTimeoutError struct{}

func (err testTimeoutError) Timeout() bool {
	return true
}

func (e testTimeoutError) Error() string { return "timeout test error" }

func (e testTimeoutError) Unwrap() error { return e }

type testListener struct {
	newConnCh   <-chan net.Conn
	returnError error
}

func (l *testListener) Close() error {
	return nil
}

func (l *testListener) Addr() net.Addr {
	return nil
}

func (l *testListener) Accept() (net.Conn, error) {
	conn := <-l.newConnCh
	var err error
	if l.returnError != nil {
		err = l.returnError
	}
	return conn, err
}

// Actual test method(s)
func TestBasicListener_Accept_Returns_OpError(t *testing.T) {
	newConnCh := make(chan net.Conn)
	errCh := make(chan error)
	errLogger := func(err error) {
		errCh <- err
	}
	listener := &testListener{
		newConnCh:   newConnCh,
		returnError: ErrStopListenerError,
	}

	connCh := make(chan connection.HandshakeConn)
	l := gateway.NewBasicListener(listener, connCh, errLogger)

	go func() {
		l.Listen()
	}()

	select {
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener called accept (this is good)")
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection")
		t.FailNow()
	}

	select {
	case err := <-errCh:
		t.Logf("Error has been received: %v (this is good)", err)
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by second select")
		t.FailNow()
	}

	select {
	case err := <-errCh:
		t.Logf("unexpected error was thrown: %v", err)
		t.FailNow()
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by third select (this is good)")
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener should have stopped")
		t.FailNow()
	}
}

func TestBasicListener_Has_Error_But_Continues(t *testing.T) {
	newConnCh := make(chan net.Conn)
	errCh := make(chan error)
	errLogger := func(err error) {
		errCh <- err
	}
	listener := &testListener{
		newConnCh:   newConnCh,
		returnError: ErrTimeoutListenerError,
	}

	connCh := make(chan connection.HandshakeConn)
	l := gateway.NewBasicListener(listener, connCh, errLogger)

	go func() {
		l.Listen()
	}()

	select {
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener called accept (this is good)")
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection")
		t.FailNow()
	}

	select {
	case err := <-errCh:
		t.Logf("Error has been received: %v (this is good...?)", err)
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by second select")
		t.FailNow()
	}

	select {
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by third select")
		t.FailNow()
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener accepts second connections (this is good)")
	}

}

func TestBasicListener_Accept_Without_Error(t *testing.T) {
	newConnCh := make(chan net.Conn)
	listener := &testListener{
		newConnCh: newConnCh,
	}

	connCh := make(chan connection.HandshakeConn)
	l := gateway.NewBasicListener(listener, connCh)

	go func() {
		l.Listen()
	}()

	select {
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener called accept (this is good)")
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection")
		t.FailNow()
	}

	select {
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by second select ")
		t.FailNow()
	case <-connCh:
		t.Log("Listener passed on received connection (this is good)")
	}

	select {
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out by second select ")
		t.FailNow()
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener called accept (this is good)")
	}

}
