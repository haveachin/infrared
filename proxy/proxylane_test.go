package proxy_test

import (
	"errors"
	"io"
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

	createHsConn = func(domain string) connection.HandshakeConn {
		hsPk := handshaking.ServerBoundHandshake{
			ServerAddress: protocol.String(domain),
			NextState:     1,
		}.Marshal()
		netConn, otherConn := net.Pipe()
		conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
		go func() {
			conn := connection.NewServerConn(otherConn)
			conn.WritePacket(hsPk)
		}()
		return conn
	}
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

func newTestListener() (func(addr string) (net.Listener, error), chan net.Conn) {
	newConnCh := make(chan net.Conn)
	netListener := &testListener{newConnCh: newConnCh}
	listenerFactory := func(addr string) (net.Listener, error) {
		return netListener, nil
	}
	return listenerFactory, newConnCh
}

func TestListenerCreation(t *testing.T) {
	listenerFactory, newConnCh := newTestListener()
	proxyCfg := proxy.NewProxyLaneConfig()
	proxyCfg.ListenerFactory = listenerFactory
	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartProxy()

	netConn, _ := net.Pipe()
	select {
	case newConnCh <- netConn:
		t.Log("Listener took connection")
	case <-time.After(defaultChTimeout):
		t.Log("Listener didnt accept connection")
		t.FailNow()
	}
}

func TestGatewayCreation(t *testing.T) {
	netConn, _ := net.Pipe()
	conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
	listenerFactory, _ := newTestListener()
	proxyCfg := proxy.NewProxyLaneConfig()
	proxyCfg.ListenerFactory = listenerFactory

	proxyLane := proxy.NewProxyLane(proxyCfg)
	toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
	proxyLane.StartProxy()

	select {
	case toGatewayCh <- conn:
		t.Log("Gateway took connection  (this is expected)")
	case <-time.After(defaultChTimeout):
		t.Log("Gateway didnt take connection (this probably means that there werent enough gateways running))")
		t.FailNow()
	}
}

func TestServerCreation(t *testing.T) {
	mainDomain := "infrared-1"
	serverCfg := server.ServerConfig{
		MainDomain: mainDomain,
	}
	listenerFactory, _ := newTestListener()
	proxyCfg := proxy.NewProxyLaneConfig()
	proxyCfg.ListenerFactory = listenerFactory
	proxyCfg.Servers = []server.ServerConfig{serverCfg}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartProxy()

	serverMap := proxyLane.TestMethod_ServerMap()
	serverCh := serverMap[mainDomain].ConnCh
	conn := createHsConn(mainDomain)

	select {
	case serverCh <- conn:
		t.Log("Server is created")
	case <-time.After(defaultChTimeout):
		t.Log("Server didnt got connection")
		t.FailNow()
	}

}

func TestServerCreation_DoesRegisterDomains(t *testing.T) {
	mainDomain := "infrared-1"
	extraDomain := "infrared-2"
	extraDomain2 := "infrared-3"

	serverCfg := server.ServerConfig{
		MainDomain:   mainDomain,
		ExtraDomains: []string{extraDomain, extraDomain2},
	}

	listenerFactory, _ := newTestListener()
	proxyCfg := proxy.NewProxyLaneConfig()
	proxyCfg.ListenerFactory = listenerFactory
	proxyCfg.Servers = []server.ServerConfig{serverCfg}

	testOrDomainIsRegistered := func(t *testing.T, testDomain string) {
		t.Helper()
		proxyLane := proxy.NewProxyLane(proxyCfg)
		proxyLane.StartProxy()

		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap := proxyLane.TestMethod_ServerMap()
		serverMap[mainDomain].CloseCh <- struct{}{}
		serverCh := serverMap[mainDomain].ConnCh
		conn := createHsConn(testDomain)
		select {
		case toGatewayCh <- conn:
			t.Log("gateway took connection  (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("gateway didnt accept connection (this probably means that there werent enough gateways running))")
			t.FailNow()
		}

		select {
		case <-serverCh:
			t.Log("Server got connection (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Server didnt get the connection")
			t.FailNow()
		}
	}

	testOrDomainIsRegistered(t, mainDomain)
	testOrDomainIsRegistered(t, extraDomain)
	testOrDomainIsRegistered(t, extraDomain2)
}

func TestProxyLane_CloseServer(t *testing.T) {
	mainDomain := "infrared-1"
	serverCfg := server.ServerConfig{
		MainDomain: mainDomain,
	}
	listenerFactory, _ := newTestListener()
	proxyCfg := proxy.NewProxyLaneConfig()
	proxyCfg.ListenerFactory = listenerFactory
	proxyCfg.Servers = []server.ServerConfig{serverCfg}

	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartProxy()
	proxyLane.CloseServer(mainDomain)

	toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
	hsPk := handshaking.ServerBoundHandshake{
		ServerAddress: protocol.String(mainDomain),
	}.Marshal()
	netConn, otherConn := net.Pipe()
	go func() {
		conn := connection.NewServerConn(otherConn)
		conn.WritePacket(hsPk)
	}()
	conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})

	select {
	case toGatewayCh <- conn:
		t.Log("Gateway took connection")
	case <-time.After(defaultChTimeout):
		t.Log("Gateway didnt take connection")
		t.FailNow()
	}

	_, err := conn.Conn().Write([]byte{1, 2, 3, 4, 5})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Logf("Error wasnt EOF but %v", err)
		t.Fail()
	}

}

func TestUpdateServer(t *testing.T) {
	mainDomain := "infrared-1"
	serverCfg := server.ServerConfig{
		MainDomain: mainDomain,
	}
	createProxyLane := func(cfg server.ServerConfig) proxy.ProxyLane {
		listenerFactory, _ := newTestListener()
		proxyCfg := proxy.NewProxyLaneConfig()
		proxyCfg.ListenerFactory = listenerFactory
		proxyCfg.Servers = []server.ServerConfig{serverCfg}

		proxyLane := proxy.NewProxyLane(proxyCfg)
		proxyLane.StartProxy()
		return proxyLane
	}

	t.Run("adding an ExtraDomain", func(t *testing.T) {
		proxyLane := createProxyLane(serverCfg)
		extraDomain := "infrared-2"
		serverCfg.ExtraDomains = []string{extraDomain}
		proxyLane.UpdateServer(serverCfg)

		serverMap := proxyLane.TestMethod_ServerMap()
		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap[mainDomain].CloseCh <- struct{}{}
		serverCh := serverMap[mainDomain].ConnCh
		select {
		case toGatewayCh <- createHsConn(extraDomain):
			t.Log("Gateway took connection")
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
			t.FailNow()
		}

		select {
		case <-serverCh:
			t.Log("Server is found")
		case <-time.After(defaultChTimeout):
			t.Log("Server wasnt found")
			t.Fail()
		}

	})

	t.Run("removing an ExtraDomain", func(t *testing.T) {
		extraDomain := "infrared-2"
		serverCfg.ExtraDomains = []string{extraDomain}
		proxyLane := createProxyLane(serverCfg)
		serverCfg.ExtraDomains = []string{}
		proxyLane.UpdateServer(serverCfg)

		serverMap := proxyLane.TestMethod_ServerMap()
		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap[mainDomain].CloseCh <- struct{}{}
		serverCh := serverMap[mainDomain].ConnCh
		conn := createHsConn(extraDomain)

		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection")
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
			t.FailNow()
		}

		select {
		case <-serverCh:
			t.Log("Server is found but shouldnt")
			t.Fail()
		case <-time.After(defaultChTimeout):
			t.Log("Server wasnt found")
		}
	})

	t.Run("keep its extra ExtraDomain", func(t *testing.T) {
		extraDomain := "infrared-2"
		serverCfg.ExtraDomains = []string{extraDomain}
		proxyLane := createProxyLane(serverCfg)
		proxyLane.UpdateServer(serverCfg)

		serverMap := proxyLane.TestMethod_ServerMap()
		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap[mainDomain].CloseCh <- struct{}{}
		serverCh := serverMap[mainDomain].ConnCh
		conn := createHsConn(extraDomain)

		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection")
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
			t.FailNow()
		}

		select {
		case <-serverCh:
			t.Log("Server is found")
		case <-time.After(defaultChTimeout):
			t.Log("Server wasnt found")
			t.Fail()
		}
	})

}
