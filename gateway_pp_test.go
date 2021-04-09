package infrared

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pires/go-proxyproto"
)

type matchIp func(ip1, ip2 string) bool

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

func createProxyProtocolConfig(portEnd int, proxyproto bool) *ProxyConfig {
	config := createBasicProxyConfig(portEnd)
	config.ProxyProtocol = proxyproto
	return config
}

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
			errorCh := make(chan *testError)
			resultCh := make(chan bool)
			shareCh := make(chan string)

			go func() {
				config := createProxyProtocolConfig(tc.portEnd, tc.proxyproto)
				if err := startGatewayWithConfig(config); err != nil {
					errorCh <- err
				}
			}()

			go func() {
				if err := startProxyProtoListen(tc.portEnd, shareCh); err != nil {
					errorCh <- err
				}
			}()

			go func() {
				same, err := startProxyProtoDial(tc.portEnd, shareCh, tc.validator)
				if err != nil {
					errorCh <- err
				}
				resultCh <- same
			}()

			select {
			case err := <-errorCh:
				t.Fatalf("Unexpected Error in test: %s\n%v", err.Message, err.Error)
			case r := <-resultCh:
				if !r {
					t.Fail()
				}
			}

		})
	}
}

func startProxyProtoListen(portEnd int, shareCh chan<- string) *testError {
	listenAddr := listenAddr(portEnd)
	listener, err := Listen(listenAddr)
	if err != nil {
		return &testError{err, fmt.Sprintf("Can't listen to %v", listenAddr)}
	}
	defer listener.Close()

	proxyListener := &proxyproto.Listener{Listener: listener.Listener}
	defer proxyListener.Close()

	conn, err := proxyListener.Accept()
	if err != nil {
		return &testError{err, "Can't accept connection on listener"}
	}
	defer conn.Close()
	ip := getIpFromAddr(conn.RemoteAddr())
	shareCh <- ip
	return nil
}

func startProxyProtoDial(portEnd int, shareCh <-chan string, validator func(ip1, ip2 string) bool) (bool, *testError) {
	time.Sleep(dialWait) // Startup time for gateway
	gatewayAddr := gatewayAddr(portEnd)
	conn, err := createConnWithFakeIP(gatewayAddr)
	if err != nil {
		return false, &testError{err, "Can't create connection"}
	}
	defer conn.Close()

	if err := sendHandshake(conn, portEnd); err != nil {
		return false, err
	}

	usedIp := getIpFromAddr(conn.LocalAddr())
	receivedIp := <-shareCh

	return validator(usedIp, receivedIp), nil
}
