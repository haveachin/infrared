package connection_test

import (
	"bytes"
	"testing"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
)

//cant import from server_test.go, so made new one
var (
	testLoginHSID  byte = 5
	testLoginID    byte = 6
	testStatusHSID byte = 10

	testUnboundID byte = 31
)

// Should implement connection.Connect
type loginTestConn struct {
	hs         protocol.Packet
	loginPK    protocol.Packet
	readCount  int
	writeCount int
}

func (c *loginTestConn) WritePacket(p protocol.Packet) error {
	c.writeCount++
	return nil
}

func (c *loginTestConn) ReadPacket() (protocol.Packet, error) {
	c.readCount++
	switch c.readCount {
	case 1:
		return c.hs, nil
	case 2:
		return c.loginPK, nil
	default:
		return protocol.Packet{}, nil
	}

}

type serverTestConn struct {
	status         protocol.Packet
	readCount      int
	writeCount     int
	receivedPacket protocol.Packet
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

func (c *serverTestConn) receivedPk() protocol.Packet {
	return c.receivedPacket
}

// util functions
func checkReceivedPk(t *testing.T, fn func() (protocol.Packet, error), expectedPK protocol.Packet) {
	pk, err := fn()
	if err != nil {
		t.Errorf("got error: %v", err)
	}

	if !samePK(expectedPK, pk) {
		t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", expectedPK, pk)
	}
}

func samePK(expected, received protocol.Packet) bool {
	sameID := expected.ID == received.ID
	sameData := bytes.Equal(expected.Data, received.Data)

	return sameID && sameData
}

// Actual test methods
func TestBasicLoginConnection(t *testing.T) {

	expectedHS := protocol.Packet{ID: testLoginHSID}
	expecedLogin := protocol.Packet{ID: testLoginID}
	conn := &loginTestConn{hs: expectedHS, loginPK: expecedLogin}

	lConn := connection.CreateBasicLoginConn(conn)

	checkReceivedPk(t, lConn.HS, expectedHS)
	checkReceivedPk(t, lConn.LoginStart, expecedLogin)

}

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

	if !samePK(expectedRequest, receivedReq) {
		t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", expectedRequest, receivedReq)
	}

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

	if !samePK(expectedRequest, receivedReq) {
		t.Errorf("Packets are different\nexpected:\t%v\ngot:\t\t%v", expectedRequest, receivedReq)
	}

}
