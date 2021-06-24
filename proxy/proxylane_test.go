package proxy_test

import (
	"net"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol"
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

func TestCreateListener(t *testing.T) {
	numberOfListeners := 3
	newConnCh := make(chan net.Conn)
	netListener := &testListener{newConnCh: newConnCh}
	listenerFactory := func(addr string) (net.Listener, error) {
		return netListener, nil
	}
	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfListeners: numberOfListeners,
		ListenerFactory:   listenerFactory,
	}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.CreateListeners()

	netConn, _ := net.Pipe()
	for i := 0; i < numberOfListeners; i++ {
		select {
		case newConnCh <- netConn:
			t.Log("Listener took connection  (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Listener didnt accept connection (this probably means that there werent enough listeners running))")
			t.FailNow()
		}
	}

	select {
	case newConnCh <- netConn:
		t.Log("Listener took connection (which probably means that there were to many servers running, or the connections before failed to told their servers busy)")
		t.FailNow()
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection (this is expected)")
	}
}

func TestCreateGateway(t *testing.T) {
	numberOfGateways := 3
	netConn, _ := net.Pipe()
	conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
	servers := []server.ServerConfig{
		{
			MainDomain: "infrared-1",
		},
	}
	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: numberOfGateways,
		Servers:          servers,
	}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()

	proxyLane.CreateGateways()

	for i := 0; i < numberOfGateways; i++ {
		select {
		case toGatewayCh <- conn:
			t.Log("Listener took connection  (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Listener didnt accept connection (this probably means that there werent enough listeners running))")
			t.FailNow()
		}
	}

	select {
	case toGatewayCh <- conn:
		t.Log("Listener took connection (which probably means that there were to many servers running, or the connections before failed to told their servers busy)")
		t.FailNow()
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection (this is expected)")
	}
}

func TestCreateGateway_DoesRegisterDomains(t *testing.T) {
	mainDomain := "infrared-1"
	extraDomain := "infrared-2"
	extraDomain2 := "infrared-3"

	createHsConn := func(domain string) connection.HandshakeConn {
		hsPk := handshaking.ServerBoundHandshake{
			ServerAddress: protocol.String(domain),
		}.Marshal()
		netConn, otherConn := net.Pipe()
		conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
		go func() {
			conn := connection.NewServerConn(otherConn)
			conn.WritePacket(hsPk)
		}()
		return conn
	}

	serverCfg := server.ServerConfig{
		NumberOfInstances: 0,
		MainDomain:        mainDomain,
		ExtraDomains:      []string{extraDomain, extraDomain2},
	}

	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: 1,
	}

	testOrDomainIsRegistered := func(t *testing.T, testDomain string) {
		t.Helper()
		proxyLane := proxy.NewProxyLane(proxyCfg)
		proxyLane.CreateGateways()
		proxyLane.RegisterServers(serverCfg)

		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap := proxyLane.TestMethod_ServerMap()
		serverCh := serverMap[mainDomain].ConnCh
		conn := createHsConn(testDomain)
		select {
		case toGatewayCh <- conn:
			t.Log("Listener took connection  (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Listener didnt accept connection (this probably means that there werent enough listeners running))")
			t.FailNow()
		}

		select {
		case <-serverCh:
			t.Log("Server got connection (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Server didnt got connection")
			t.FailNow()
		}
	}

	testOrDomainIsRegistered(t, mainDomain)
	testOrDomainIsRegistered(t, extraDomain)
	testOrDomainIsRegistered(t, extraDomain2)
}

func TestProxyLane_InitialServerSetup_CreatesRightAmountOfServers(t *testing.T) {
	numberOfInstances := 2
	hs := handshaking.ServerBoundHandshake{
		NextState: 1, //Using status so it first expects another request before making the server connection
	}
	netConn, _ := net.Pipe()
	conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
	conn.Handshake = hs
	singleServerCfg := server.ServerConfig{
		MainDomain:        "infrared-1",
		NumberOfInstances: numberOfInstances,
	}

	proxyCfg := proxy.ProxyLaneConfig{}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.RegisterServers(singleServerCfg)

	serverMap := proxyLane.TestMethod_ServerMap()
	serverCh := serverMap[singleServerCfg.MainDomain].ConnCh

	for i := 0; i < numberOfInstances; i++ {
		t.Logf("for loop run: %d\n", i)
		select {
		case <-time.After(defaultChTimeout):
			t.Log("Tasked timed out (this probably means that there werent enough servers running)")
			t.FailNow()
		case serverCh <- conn:
			t.Log("a server took the connection")
		}
	}

	select {
	case <-time.After(defaultChTimeout):
		t.Log("Tasked timed out (which means that there were no unexpected extra servers running)")
	case serverCh <- connection.NewHandshakeConn(nil, nil):
		t.Log("a server took the connection (which probably means that there were to many servers running, or the connections before failed to told their servers busy)")
		t.FailNow()
	}

}
