package connection_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
)

var (
	testLoginHSID  byte = 5
	testLoginID    byte = 6
	testStatusHSID byte = 10

	testUnboundID byte = 31

	testSendID    byte = 41
	testReceiveID byte = 42
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

	testSamePK(t, expectedPK, pk)
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
func TestBasicLoginConnection(t *testing.T) {

	expectedHS := protocol.Packet{ID: testLoginHSID}
	expecedLogin := protocol.Packet{ID: testLoginID}
	conn := &loginTestConn{hs: expectedHS, loginPK: expecedLogin}

	lConn := connection.CreateBasicLoginConn(conn)

	checkReceivedPk(t, lConn.HsPk, expectedHS)
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
