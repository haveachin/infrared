package proxy_test

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/proxy"
	"github.com/haveachin/infrared/server"
)

var (
	defaultChanTimeout = 50 * time.Millisecond
)

func init() {
	if timeStr := os.Getenv("CHANNEL_TIMEOUT"); timeStr != "" {
		duration, err := time.ParseDuration(timeStr)
		if err == nil {
			defaultChanTimeout = duration
		}
	}

}

type testListener struct {
	conn             net.Conn
	startConnections bool
	count            int
}

func (l *testListener) Close() error {
	return nil
}

func (l *testListener) Addr() net.Addr {
	return nil
}

func (l *testListener) Accept() (net.Conn, error) {
	for !l.startConnections {
		//Not really necessary but prevents some unnecessary computations
		time.Sleep(1 * time.Millisecond)
	}
	l.count++
	return l.conn, nil
}

func TestProxyLane_ListenersCreation(t *testing.T) {
	numberOfListeners := 3
	c1, _ := net.Pipe()
	netListener := &testListener{conn: c1, startConnections: false}
	listenerFactory := func(addr string) (net.Listener, error) {
		return netListener, nil
	}
	proxyLaneCfg := proxy.ProxyLaneConfig{
		NumberOfListeners: numberOfListeners,
		ListenerFactory:   listenerFactory,
	}
	toGatewayChan := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.HandleListeners(toGatewayChan)

	netListener.startConnections = true

	// Just wait for some time
	<-time.After(defaultChanTimeout)

	if netListener.count != numberOfListeners {
		t.Error("different number of connections have been opened than we expected")
		t.Logf("expected:\t%v", numberOfListeners)
		t.Logf("got:\t\t%v", netListener.count)
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
			DomainName: "infrared-1",
		},
	}

	toGatewayChan := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.LoadServers(servers)
	proxyLane.HandleGateways(toGatewayChan)
	for i := 0; i < numberOfGateways; i++ {
		toGatewayChan <- hsConn
	}

	select {
	case <-time.After(defaultChanTimeout):
		t.Log("channel didnt took in another connection which was meant to be")
	case toGatewayChan <- hsConn:
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
		DomainName:        "infrared-1",
		NumberOfInstances: numberOfInstances,
	}

	servers := []server.ServerConfig{
		singleServerCfg,
		{
			DomainName:        "infrared-2",
			NumberOfInstances: 3,
		},
	}

	toGatewayChan := make(chan connection.HandshakeConn)

	proxyLane := proxy.ProxyLane{Config: proxyLaneCfg}

	proxyLane.LoadServers(servers)
	proxyLane.HandleGateways(toGatewayChan)
	proxyLane.HandleServer(singleServerCfg)

	numberfOfChannelAccepts := numberOfInstances + numberOfGateways
	runsNeededToTest := numberOfInstances + numberOfGateways + 1
	for i := 1; i <= runsNeededToTest; i++ {
		t.Logf("run: %d\n", i)
		select {
		case <-time.After(defaultChanTimeout):
			if i <= numberfOfChannelAccepts {
				t.Log("channel stop taking in connections earlier than expected")
				t.FailNow()
			} else {
				t.Log("channel didnt took in another connection which was expected to happen")
			}
		case toGatewayChan <- createConn():
			if i > numberfOfChannelAccepts {
				t.Error("Tasked should have timed out but didnt")
			}
			t.Log("channel took in connection")
		}
	}

}
