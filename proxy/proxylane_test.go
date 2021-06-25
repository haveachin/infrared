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

func TestListenerCreation(t *testing.T) {
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
	proxyLane.StartupProxy()

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

func TestGatewayCreation(t *testing.T) {
	numberOfGateways := 3
	netConn, _ := net.Pipe()
	conn := connection.NewHandshakeConn(netConn, &net.IPAddr{})
	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: numberOfGateways,
	}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()

	proxyLane.StartupProxy()

	for i := 0; i < numberOfGateways; i++ {
		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection  (this is expected)")
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection (this probably means that there werent enough gateways running))")
			t.FailNow()
		}
	}

	select {
	case toGatewayCh <- conn:
		t.Log("Gateway took connection (which probably means that there were to many gateways running, or the connections before failed to told their gateways busy)")
		t.FailNow()
	case <-time.After(defaultChTimeout):
		t.Log("Gateway didnt take connection (this is expected)")
	}
}

func TestServerCreation(t *testing.T) {
	mainDomain := "infrared-1"
	serverCfg := server.ServerConfig{
		NumberOfInstances: 1,
		MainDomain:        mainDomain,
	}
	proxyCfg := proxy.ProxyLaneConfig{
		Servers: []server.ServerConfig{serverCfg},
	}

	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartupProxy()

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
		NumberOfInstances: 0,
		MainDomain:        mainDomain,
		ExtraDomains:      []string{extraDomain, extraDomain2},
	}

	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: 1,
		Servers:          []server.ServerConfig{serverCfg},
	}

	testOrDomainIsRegistered := func(t *testing.T, testDomain string) {
		t.Helper()
		proxyLane := proxy.NewProxyLane(proxyCfg)
		proxyLane.StartupProxy()

		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		serverMap := proxyLane.TestMethod_ServerMap()
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

func TestProxyLane_ServerCreation_CreatesRightAmountOfServers(t *testing.T) {
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

	proxyCfg := proxy.ProxyLaneConfig{
		Servers: []server.ServerConfig{singleServerCfg},
	}
	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartupProxy()

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

func TestProxyLane_CloseServer(t *testing.T) {
	mainDomain := "infrared-1"
	serverCfg := server.ServerConfig{
		NumberOfInstances: 1,
		MainDomain:        mainDomain,
	}
	proxyCfg := proxy.ProxyLaneConfig{
		NumberOfGateways: 1,
		Servers:          []server.ServerConfig{serverCfg},
	}

	proxyLane := proxy.NewProxyLane(proxyCfg)
	proxyLane.StartupProxy()
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
		proxyCfg := proxy.ProxyLaneConfig{
			NumberOfGateways: 1,
			Servers:          []server.ServerConfig{cfg},
		}

		proxyLane := proxy.NewProxyLane(proxyCfg)
		proxyLane.StartupProxy()
		return proxyLane
	}

	t.Run("adding an ExtraDomain", func(t *testing.T) {
		proxyLane := createProxyLane(serverCfg)
		extraDomain := "infrared-2"
		serverCfg.ExtraDomains = []string{extraDomain}
		proxyLane.UpdateServer(serverCfg)

		serverMap := proxyLane.TestMethod_ServerMap()
		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
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

	t.Run("Keep its extra ExtraDomain", func(t *testing.T) {
		extraDomain := "infrared-2"
		serverCfg.ExtraDomains = []string{extraDomain}
		proxyLane := createProxyLane(serverCfg)
		proxyLane.UpdateServer(serverCfg)

		serverMap := proxyLane.TestMethod_ServerMap()
		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
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

	t.Run("Increases number of running servers", func(t *testing.T) {
		proxyLane := createProxyLane(serverCfg)
		numberOfInstances := 1
		serverCfg.NumberOfInstances = numberOfInstances
		proxyLane.UpdateServer(serverCfg)

		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		conn := createHsConn(mainDomain)

		for i := 0; i < numberOfInstances+1; i++ {
			select {
			case toGatewayCh <- conn:
				t.Log("Gateway took connection")
			case <-time.After(defaultChTimeout):
				// If this times out the other server isnt running since there only is 1 gateway
				t.Log("Gateway didnt take connection")
				t.FailNow()
			}
		}

		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection again (while it shouldnt since there is only one gateway and no one should receive the first connection")
			t.FailNow()
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
		}

	})

	t.Run("decreases number of running servers", func(t *testing.T) {
		serverCfg.NumberOfInstances = 1
		proxyLane := createProxyLane(serverCfg)
		serverCfg.NumberOfInstances = 0
		proxyLane.UpdateServer(serverCfg)

		toGatewayCh, _ := proxyLane.TestMethod_GatewayCh()
		conn := createHsConn(mainDomain)

		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection")
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
			t.FailNow()
		}

		select {
		case toGatewayCh <- conn:
			t.Log("Gateway took connection again (while it shouldnt since there is only one gateway and no one should receive the first connection")
			t.FailNow()
		case <-time.After(defaultChTimeout):
			t.Log("Gateway didnt take connection")
		}
	})

}
