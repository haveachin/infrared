package infrared

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pires/go-proxyproto"
)

var serverAddr string = "infrared.gateway"

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

type matchIp func(ip1, ip2 string) bool

func TestProxyProtocol(t *testing.T) {
	tt := []struct {
		name       string
		proxyproto bool
		portEnd    int
		validator  matchIp
	}{
		{
			name:       "ProxyProtocolOn",
			proxyproto: true,
			portEnd:    581,
			validator: func(ip1, ip2 string) bool {
				return ip1 == ip2
			},
		},
		{
			name:       "ProxyProtocolOff",
			proxyproto: false,
			portEnd:    582,
			validator: func(ip1, ip2 string) bool {
				return ip1 != ip2
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cDial := make(chan error)
			cListen := make(chan error)
			cGateway := make(chan error)
			cResult := make(chan bool)
			cInfo := make(chan string)

			config := createProxyProtocolConfig(tc.portEnd, tc.proxyproto)
			startGatewayWithConfig(config, cGateway)

			startProxyProtoListen(tc.portEnd, cListen, cInfo)
			startProxyProtoDial(tc.portEnd, cListen, cResult, cInfo, tc.validator)

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

		})
	}
}

func createProxyProtocolConfig(portEnd int, proxyproto bool) *ProxyConfig {
	config := createBasicProxyConfig(portEnd)
	config.ProxyProtocol = proxyproto
	return config
}

func startProxyProtoListen(portEnd int, errCh chan<- error, shareCh chan<- string) {
	go func() {
		listenAddr := portToAddr(listenPort(portEnd))
		listener, err := Listen(listenAddr)
		if err != nil {
			errCh <- err
			return
		}
		defer listener.Close()

		proxyListener := &proxyproto.Listener{Listener: listener.Listener}
		defer proxyListener.Close()

		conn, err := proxyListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		ip := getIpFromAddr(conn.RemoteAddr())
		shareCh <- ip
	}()
}

func startProxyProtoDial(portEnd int, errCh chan<- error, resultCh chan<- bool, shareCh <-chan string, validator func(ip1, ip2 string) bool) {
	go func() {

		time.Sleep(dialWait) // Startup time for gateway
		gatewayPort := gatewayPort(portEnd)
		gatewayAddr := portToAddr(gatewayPort)
		conn, err := createConnWithFakeIP(gatewayAddr)
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		pk := createStatusHankshake(portEnd)
		if err := conn.WritePacket(pk); err != nil {
			errCh <- err
			return
		}
		usedIp := getIpFromAddr(conn.LocalAddr())
		receivedIp := <-shareCh

		resultCh <- validator(usedIp, receivedIp)
	}()
}
