package infrared

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
	"github.com/pires/go-proxyproto"
)

var serverAddr = "infrared.gateway"
var dialWait = 5 * time.Millisecond // Startup time for gateway

type testError struct {
	Error   error
	Message string
}

func listenPort(portEnd int) int {
	return 20000 + portEnd
}

func gatewayPort(portEnd int) int {
	return 30000 + portEnd
}

func gatewayAddr(portEnd int) string {
	return portToAddr(gatewayPort(portEnd))
}

func listenAddr(portEnd int) string {
	return portToAddr(listenPort(portEnd))
}

func portToAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

func createBasicProxyConfig(portEnd int) *ProxyConfig {
	listenAddr := listenAddr(portEnd)
	gatewayAddr := gatewayAddr(portEnd)

	return &ProxyConfig{
		DomainName: serverAddr,
		ListenTo:   gatewayAddr,
		ProxyTo:    listenAddr,
	}
}

func createStatusHandshake(portEnd int) protocol.Packet {
	gatewayPort := gatewayPort(portEnd)
	hs := handshaking.ServerBoundHandshake{
		ProtocolVersion: 574,
		ServerAddress:   protocol.String(serverAddr),
		ServerPort:      protocol.UnsignedShort(gatewayPort),
		NextState:       1,
	}
	return hs.Marshal()
}

func createProxyProtocolHeader() proxyproto.Header {
	return proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("127.0.10.1"),
			Port: 0,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("127.0.20.1"),
			Port: 0,
		},
	}
}

func startGatewayWithConfig(config *ProxyConfig) *testError {
	var proxies []*Proxy
	proxy := &Proxy{Config: config}
	proxies = append(proxies, proxy)

	return startGatewayProxies(proxies)
}

func startGatewayProxies(proxies []*Proxy) *testError {
	gateway := Gateway{}
	if err := gateway.ListenAndServe(proxies); err != nil {
		return &testError{err, "Can't start gateway"}
	}
	return nil
}

func sendHandshake(conn Conn, portEnd int) *testError {
	pk := createStatusHandshake(portEnd)
	if err := conn.WritePacket(pk); err != nil {
		return &testError{err, "Can't write handshake"}
	}
	return nil
}

func sendProxyProtocolHeader(rconn Conn) *testError {
	header := createProxyProtocolHeader()
	if _, err := header.WriteTo(rconn); err != nil {
		return &testError{err, "Can't write proxy protocol header"}
	}
	return nil
}

var serverVersionName = "Infrared-test-online"
var samples = make([]PlayerSample, 0)
var statusConfig = StatusConfig{VersionName: serverVersionName, ProtocolNumber: 754,
	MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}

var onlineStatus = StatusConfig{
	VersionName:    "Infrared 1.16.5 Online",
	ProtocolNumber: 754,
	MaxPlayers:     20,
	MOTD:           "Powered by Infrared",
}

var offlineStatus = StatusConfig{
	VersionName:    "Infrared 1.16.5 Offline",
	ProtocolNumber: 754,
	MaxPlayers:     20,
	MOTD:           "Powered by Infrared",
}

func startStatusListen(portEnd int) *testError {
	listenAddr := listenAddr(portEnd)
	listener, err := Listen(listenAddr)
	if err != nil {
		return &testError{err, fmt.Sprintf("Can't listen to %v", listenAddr)}
	}
	defer listener.Close()

	conn, err := listener.Accept()
	if err != nil {
		return &testError{err, "Can't accept connection on listener"}
	}
	defer conn.Close()

	pk, err := statusConfig.StatusResponsePacket()
	if err != nil {
		return &testError{err, "Can't create status response packet"}
	}
	if err := conn.WritePacket(pk); err != nil {
		return &testError{err, "Can't write status response packet on connection"}
	}
	return nil
}

func startStatusDial(portEnd int, useProxyProtocolHeader bool, expectedName string) (bool, *testError) {
	time.Sleep(dialWait) // Startup time for gateway
	gatewayAddr := gatewayAddr(portEnd)
	conn, err := Dial(gatewayAddr)
	if err != nil {
		return false, &testError{err, "Can't make a connection with gateway"}
	}
	defer conn.Close()

	if useProxyProtocolHeader {
		if err := sendProxyProtocolHeader(conn); err != nil {
			return false, err
		}
	}

	if err := sendHandshake(conn, portEnd); err != nil {
		return false, err
	}

	statusPk := status.ServerBoundRequest{}.Marshal()
	if err := conn.WritePacket(statusPk); err != nil {
		return false, &testError{err, "Can't write status request packet"}
	}

	receivedPk, err := conn.ReadPacket()
	if err != nil {
		return false, &testError{err, "Can't read status reponse packet"}
	}

	response, err := status.UnmarshalClientBoundResponse(receivedPk)
	if err != nil {
		return false, &testError{err, "Can't unmarshal status reponse packet"}
	}

	res := &status.ResponseJSON{}
	json.Unmarshal([]byte(response.JSONResponse), &res)
	return expectedName == res.Version.Name, nil
}

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

func TestStatusRequest(t *testing.T) {
	tt := []struct {
		name                   string
		portEnd                int
		onlineStatus           StatusConfig
		offlineStatus          StatusConfig
		activeServer           bool
		expectedVersion        string
		useProxyProtocolHeader bool
	}{
		{
			name:                   "ServerOnlineWithoutConfigAndWithoutProxyProtocolHeader",
			portEnd:                570,
			activeServer:           true,
			expectedVersion:        serverVersionName,
			useProxyProtocolHeader: false,
		},
		{
			name:                   "ServerOfflineWithoutConfigWithoutProxyProtocolHeader",
			portEnd:                571,
			activeServer:           false,
			expectedVersion:        "",
			useProxyProtocolHeader: false,
		},
		{
			name:                   "ServerOnlineWithConfigWithoutProxyProtocolHeader",
			portEnd:                572,
			onlineStatus:           onlineStatus,
			offlineStatus:          offlineStatus,
			activeServer:           true,
			expectedVersion:        onlineStatus.VersionName,
			useProxyProtocolHeader: false,
		},
		{
			name:                   "ServerOfflineWithConfigWithProxyProtocolHeader",
			portEnd:                573,
			onlineStatus:           onlineStatus,
			offlineStatus:          offlineStatus,
			activeServer:           false,
			expectedVersion:        offlineStatus.VersionName,
			useProxyProtocolHeader: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errorCh := make(chan *testError)
			resultCh := make(chan bool)

			go func() {
				config := createBasicProxyConfig(tc.portEnd)
				config.OnlineStatus = tc.onlineStatus
				config.OfflineStatus = tc.offlineStatus

				if err := startGatewayWithConfig(config); err != nil {
					errorCh <- err
				}
			}()

			if tc.activeServer {
				go func() {
					if err := startStatusListen(tc.portEnd); err != nil {
						errorCh <- err
					}
				}()
			}

			go func() {
				same, err := startStatusDial(tc.portEnd, tc.useProxyProtocolHeader, tc.expectedVersion)
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
