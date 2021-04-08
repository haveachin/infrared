package infrared

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/pires/go-proxyproto"
)

// Checking or proxy is working by checking or the remoteAddr's port is different from the port the client is using
func getIpFromAddr(addr net.Addr) string {
	return strings.Split(addr.String(), ":")[0]
}

func createConnWithFakeIP(gatewayAddr string) (Conn, error) {
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP("127.0.10.1"),
			Port: 0,
		},
	}
	netConn, _ := dialer.Dial("tcp", gatewayAddr)
	return wrapConn(netConn), nil
}

func TestProxyProtocolOn(t *testing.T) {
	serverAddr := "infrared.gateway"
	listenAddr := ":26572"
	gatewayPort := 25572
	gatewayAddr := fmt.Sprintf(":%d", gatewayPort)

	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)
	cInfo := make(chan string)

	go func() {
		config := &ProxyConfig{
			DomainName:    serverAddr,
			ListenTo:      gatewayAddr,
			ProxyTo:       listenAddr,
			ProxyProtocol: true,
		}

		var proxies []*Proxy
		proxy := &Proxy{Config: config}
		proxies = append(proxies, proxy)
		gateway := Gateway{}

		if err := gateway.ListenAndServe(proxies); err != nil {
			cGateway <- err
			return
		}
	}()

	go func() {
		listener, err := Listen(listenAddr)
		if err != nil {
			cListen <- err
			return
		}
		defer listener.Close()

		proxyListener := &proxyproto.Listener{Listener: listener.Listener}
		defer proxyListener.Close()

		conn, err := proxyListener.Accept()
		if err != nil {
			cListen <- err
			return
		}
		defer conn.Close()

		ip := getIpFromAddr(conn.RemoteAddr())

		cInfo <- ip

	}()

	go func() {
		time.Sleep(time.Second * 1) // Startup time for gateway
		conn, err := createConnWithFakeIP(gatewayAddr)
		if err != nil {
			cDial <- err
			return
		}
		defer conn.Close()

		isHandshakePk := handshaking.ServerBoundHandshake{
			ProtocolVersion: 754,
			ServerAddress:   protocol.String(serverAddr),
			ServerPort:      protocol.UnsignedShort(gatewayPort),
			NextState:       1,
		}

		handshakePk := isHandshakePk.Marshal()

		if err := conn.WritePacket(handshakePk); err != nil {
			cDial <- err
			return
		}

		usedIp := getIpFromAddr(conn.LocalAddr())
		receivedIp := <-cInfo

		cResult <- receivedIp == usedIp

	}()

	select {
	case d := <-cDial:
		t.Fatalf("Unexpected Error in dial, this probably means that the test is bad: %v", d)
	case l := <-cListen:
		t.Fatalf("Unexpected Error in server, this probably means that the test is bad or the 'server' cant process the sent packet: %v", l)
	case g := <-cGateway:
		t.Fatalf("Unexpected Error in gateway: %v", g)
	case r := <-cResult:
		if !r {
			t.Fail()
		}
	}

}

func TestProxyProtocolOff(t *testing.T) {
	serverAddr := "infrared.gateway"
	listenAddr := ":26573"
	gatewayPort := 25573
	gatewayAddr := fmt.Sprintf(":%d", gatewayPort)

	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)
	cInfo := make(chan string)

	go func() {
		config := &ProxyConfig{
			DomainName: serverAddr,
			ListenTo:   gatewayAddr,
			ProxyTo:    listenAddr,
		}

		var proxies []*Proxy
		proxy := &Proxy{Config: config}
		proxies = append(proxies, proxy)
		gateway := Gateway{}

		if err := gateway.ListenAndServe(proxies); err != nil {
			cGateway <- err
			return
		}
	}()

	go func() {
		listener, err := Listen(listenAddr)
		if err != nil {
			cListen <- err
			return
		}
		defer listener.Close()

		proxyListener := &proxyproto.Listener{Listener: listener.Listener}
		defer proxyListener.Close()

		conn, err := proxyListener.Accept()
		if err != nil {
			cListen <- err
			return
		}
		defer conn.Close()

		ip := getIpFromAddr(conn.RemoteAddr())

		cInfo <- ip

	}()

	go func() {
		time.Sleep(time.Second * 1) // Startup time for gateway
		conn, err := createConnWithFakeIP(gatewayAddr)
		if err != nil {
			cDial <- err
			return
		}
		defer conn.Close()

		isHandshakePk := handshaking.ServerBoundHandshake{
			ProtocolVersion: 754,
			ServerAddress:   protocol.String(serverAddr),
			ServerPort:      protocol.UnsignedShort(gatewayPort),
			NextState:       1,
		}

		handshakePk := isHandshakePk.Marshal()

		if err := conn.WritePacket(handshakePk); err != nil {
			cDial <- err
			return
		}

		usedIp := getIpFromAddr(conn.LocalAddr())
		receivedIp := <-cInfo

		cResult <- receivedIp != usedIp

	}()

	select {
	case d := <-cDial:
		t.Fatalf("Unexpected Error in dial, this probably means that the test is bad: %v", d)
	case l := <-cListen:
		t.Fatalf("Unexpected Error in server, this probably means that the test is bad or the 'server' cant process the sent packet: %v", l)
	case g := <-cGateway:
		t.Fatalf("Unexpected Error in gateway: %v", g)
	case r := <-cResult:
		if !r {
			t.Fail()
		}
	}

}
