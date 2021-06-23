package proxy_test

import (
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/proxy"
	"github.com/haveachin/infrared/server"
)

var (
	defaultChTimeout = 10 * time.Millisecond
)

type testListener struct {
	newConnCh <-chan net.Conn
}

func (l *testListener) Close() error {
	return nil
}

func (l *testListener) Addr() net.Addr {
	return nil
}

func (l *testListener) Accept() (net.Conn, error) {
	conn := <-l.newConnCh
	return conn, nil
}

func TestProxyLane_ListenersCreation(t *testing.T) {
	numberOfListeners := 3
	newConnCh := make(chan net.Conn)
	netListener := &testListener{newConnCh: newConnCh}
	listenerFactory := func(addr string) (net.Listener, error) {
		return netListener, nil
	}
	proxyLaneCfg := proxy.ProxyLaneConfig{
		NumberOfListeners: numberOfListeners,
		ListenerFactory:   listenerFactory,
	}
	toGatewayCh := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.HandleListeners(toGatewayCh)
	for i := 0; i < numberOfListeners; i++ {
		newConnCh <- &net.TCPConn{}
	}

	select {
	case newConnCh <- &net.TCPConn{}:
		t.Log("Listener called accept")
		t.FailNow()
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection (this is good)")
	}

}

func TestProxyLane_GatewayCreation(t *testing.T) {
	numberOfGateways := 2
	c1, _ := net.Pipe()
	hsConn := connection.NewHandshakeConn(c1, nil)

	proxyLaneCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: numberOfGateways,
	}

	servers := []server.ServerConfig{
		{
			MainDomain: "infrared-1",
		},
	}

	toGatewayCh := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.RegisterMultipleServers(servers)
	proxyLane.HandleGateways(toGatewayCh)
	for i := 0; i < numberOfGateways; i++ {
		toGatewayCh <- hsConn
	}

	select {
	case <-time.After(defaultChTimeout):
		t.Log("channel didnt took in another connection which was meant to be")
	case toGatewayCh <- hsConn:
		t.Error("Tasked should have timed out but didnt")
	}

}

func TestProxyLane_ServerCreation(t *testing.T) {
	numberOfInstances := 2
	numberOfGateways := 1
	hsPk := handshaking.ServerBoundHandshake{
		ServerAddress: "infrared-1",
		//Using status so it first expects another request before making the server connection
		NextState: 1,
	}.Marshal()

	createConn := func() connection.HandshakeConn {
		c1, c2 := net.Pipe()
		conn := connection.NewServerConn(c1)
		go func() {
			conn.WritePacket(hsPk)
		}()

		conn2 := connection.NewHandshakeConn(c2, nil)
		return conn2
	}

	proxyLaneCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: numberOfGateways,
	}

	singleServerCfg := server.ServerConfig{
		MainDomain:        "infrared-1",
		NumberOfInstances: numberOfInstances,
	}

	servers := []server.ServerConfig{
		singleServerCfg,
		{
			MainDomain:        "infrared-2",
			NumberOfInstances: 3,
		},
	}

	toGatewayCh := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.RegisterMultipleServers(servers)
	proxyLane.HandleGateways(toGatewayCh)
	proxyLane.InitialServerSetup(singleServerCfg)

	numberfOfChAccepts := numberOfInstances + numberOfGateways
	runsNeededToTest := numberOfInstances + numberOfGateways + 1
	for i := 1; i <= runsNeededToTest; i++ {
		t.Logf("run: %d\n", i)
		select {
		case <-time.After(defaultChTimeout):
			if i <= numberfOfChAccepts {
				t.Log("channel stop taking in connections earlier than expected")
				t.FailNow()
			} else {
				t.Log("channel didnt took in another connection which was expected to happen")
			}
		case toGatewayCh <- createConn():
			if i > numberfOfChAccepts {
				t.Error("Tasked should have timed out but didnt")
			}
			t.Log("channel took in connection")
		}
	}

}
