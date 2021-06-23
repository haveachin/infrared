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
	ErrNotImplemented = errors.New("not implemented")
	ErrNoReadLeft     = errors.New("no packets left to read")

	defaultChTimeout time.Duration = 10 * time.Millisecond
)

type GatewayRunner func(gwCh <-chan connection.HandshakeConn) <-chan connection.HandshakeConn

// Actual test functions
func TestFindMatchingServer_SingleServerStore(t *testing.T) {
	serverAddr := "infrared-1"

	gatewayRunner := func(gwCh <-chan connection.HandshakeConn) <-chan connection.HandshakeConn {
		connCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: connCh}
		serverStore := &gateway.SingleServerStore{Server: serverData}

		gw := gateway.NewBasicGatewayWithStore(serverStore, gwCh, nil)
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
		serverStore := gateway.CreateDefaultServerStore()
		for id := 2; id < 10; id++ {
			serverAddr := fmt.Sprintf("addr-%d", id)
			serverData := gateway.ServerData{ConnCh: make(chan connection.HandshakeConn)}
			serverStore.AddServer(serverAddr, serverData)
		}
		connCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: connCh}

		serverStore.AddServer(serverAddr, serverData)

		gw := gateway.NewBasicGatewayWithStore(&serverStore, gwCh, nil)
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
			hsConn := connection.NewHandshakeConn(c1, addr)
			go func() {
				pk := hs.Marshal()
				bytes, _ := pk.Marshal()
				c2.Write(bytes)
			}()

			gwCh := make(chan connection.HandshakeConn)
			serverCh := data.runGateway(gwCh)

			select {
			case <-time.After(defaultChTimeout):
				t.Log("Tasked timed out")
				t.FailNow() // Dont check other code it didnt finish anyway
			case gwCh <- hsConn:
				t.Log("Gateway took connection")
			}

			select {
			case <-time.After(defaultChTimeout): //Be fast or fail >:)
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

func TestBasicGateway(t *testing.T) {

	t.Run("Test or gateway can accept connection", func(t *testing.T) {
		serverCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: serverCh}
		serverStore := &gateway.SingleServerStore{Server: serverData}

		connCh := make(chan connection.HandshakeConn)
		closeCh := make(chan struct{})

		gw := gateway.NewBasicGatewayWithStore(serverStore, connCh, closeCh)
		go func() {
			gw.Start()
		}()

		hs := handshaking.ServerBoundHandshake{ServerAddress: "infrared"}
		c1, c2 := net.Pipe()
		handshakeConn := connection.NewHandshakeConn(c1, nil)
		go func() {
			pk := hs.Marshal()
			bytes, _ := pk.Marshal()
			c2.Write(bytes)
		}()

		connCh <- handshakeConn
		select {
		case <-time.After(defaultChTimeout):
			t.Log("Tasked timed out")
			t.FailNow()
		case <-serverCh:
			t.Log("gateway received and processed connection succesfully")
		}
	})

	t.Run("Test or gateway can be closed", func(t *testing.T) {
		serverCh := make(chan connection.HandshakeConn)
		serverData := gateway.ServerData{ConnCh: serverCh}
		serverStore := &gateway.SingleServerStore{Server: serverData}

		connCh := make(chan connection.HandshakeConn)
		closeCh := make(chan struct{})

		gw := gateway.NewBasicGatewayWithStore(serverStore, connCh, closeCh)
		go func() {
			gw.Start()
		}()
		handshakeConn := connection.NewHandshakeConn(nil, nil)

		closeCh <- struct{}{}
		select {
		case <-time.After(defaultChTimeout):
			t.Log("Everything is fine the task timed out like it should have")
		case connCh <- handshakeConn:
			t.Log("Tasked should have timed out")
			t.FailNow()
		}
	})

}
