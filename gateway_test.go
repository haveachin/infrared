package infrared

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
	"github.com/pires/go-proxyproto"
)

var serverDomain string = "infrared.gateway"

type testError struct {
	Error   error
	Message string
}

func gatewayPort(portEnd int) int {
	return 30000 + portEnd
}

func gatewayAddr(portEnd int) string {
	return portToAddr(gatewayPort(portEnd))
}

func serverPort(portEnd int) int {
	return 20000 + portEnd
}

func serverAddr(portEnd int) string {
	return portToAddr(serverPort(portEnd))
}

func portToAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

func proxyConfigWithPortEnd(portEnd int) *ProxyConfig {
	serverAddr := serverAddr(portEnd)
	gatewayAddr := gatewayAddr(portEnd)
	return createBasicProxyConfig(serverDomain, gatewayAddr, serverAddr)
}

func createBasicProxyConfig(serverDomain, gatewayAddr, serverAddr string) *ProxyConfig {
	return &ProxyConfig{
		DomainName: serverDomain,
		ListenTo:   gatewayAddr,
		ProxyTo:    serverAddr,
	}
}

func createStatusHandshake(portEnd int) protocol.Packet {
	gatewayPort := gatewayPort(portEnd)
	return serverHandshake(serverDomain, gatewayPort)
}

func serverHandshake(domain string, port int) protocol.Packet {
	hs := handshaking.ServerBoundHandshake{
		ProtocolVersion: 574,
		ServerAddress:   protocol.String(domain),
		ServerPort:      protocol.UnsignedShort(port),
		NextState:       1,
	}
	return hs.Marshal()
}

func configToProxies(config *ProxyConfig) []*Proxy {
	proxyConfigs := make([]*ProxyConfig, 0)
	proxyConfigs = append(proxyConfigs, config)
	return configsToProxies(proxyConfigs)
}

func configsToProxies(config []*ProxyConfig) []*Proxy {
	var proxies []*Proxy
	for _, c := range config {
		proxy := &Proxy{Config: c}
		proxies = append(proxies, proxy)
	}
	return proxies
}

func sendHandshakePort(conn Conn, portEnd int) *testError {
	pk := createStatusHandshake(portEnd)
	return sendHandshake(conn, pk)
}

func sendHandshake(conn Conn, pk protocol.Packet) *testError {
	if err := conn.WritePacket(pk); err != nil {
		return &testError{err, "Can't write handshake"}
	}
	return nil
}

func statusPKWithVersion(name string) StatusConfig {
	samples := make([]PlayerSample, 0)
	return StatusConfig{VersionName: name, ProtocolNumber: 754,
		MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}
}

var serverVersionName string = "Infrared-test-online"

var onlineStatus StatusConfig = StatusConfig{
	VersionName:    "Infrared 1.16.5 Online",
	ProtocolNumber: 754,
	MaxPlayers:     20,
	MOTD:           "Powered by Infrared",
}

var offlineStatus StatusConfig = StatusConfig{
	VersionName:    "Infrared 1.16.5 Offline",
	ProtocolNumber: 754,
	MaxPlayers:     20,
	MOTD:           "Powered by Infrared",
}

func TestStatusRequest(t *testing.T) {
	tt := []struct {
		name            string
		portEnd         int
		onlineStatus    StatusConfig
		offlineStatus   StatusConfig
		activeServer    bool
		expectedVersion string
	}{
		{
			name:            "ServerOnlineWithoutConfig",
			portEnd:         570,
			activeServer:    true,
			expectedVersion: serverVersionName,
		},
		{
			name:            "ServerOfflineWithoutConfig",
			portEnd:         571,
			activeServer:    false,
			expectedVersion: "",
		},
		{
			name:            "ServerOnlineWithConfig",
			portEnd:         572,
			onlineStatus:    onlineStatus,
			offlineStatus:   offlineStatus,
			activeServer:    true,
			expectedVersion: onlineStatus.VersionName,
		},
		{
			name:            "ServerOfflineWithConfig",
			portEnd:         573,
			onlineStatus:    onlineStatus,
			offlineStatus:   offlineStatus,
			activeServer:    false,
			expectedVersion: offlineStatus.VersionName,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			wg := &sync.WaitGroup{}
			errorCh := make(chan *testError)
			resultCh := make(chan bool)
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				config := proxyConfigWithPortEnd(tc.portEnd)
				config.OnlineStatus = tc.onlineStatus
				config.OfflineStatus = tc.offlineStatus

				gateway := Gateway{}
				proxies := configToProxies(config)
				if err := gateway.ListenAndServe(proxies); err != nil {
					errorCh <- &testError{err, "Can't start gateway"}
				}
				wg.Done()
				gateway.KeepProcessActive()
			}(wg)

			if tc.activeServer {
				wg.Add(1)
				serverC := statusListenerConfig{}
				serverC.status = statusPKWithVersion(serverVersionName)
				serverC.addr = serverAddr(tc.portEnd)
				go func() {
					statusListen(serverC, errorCh)
					wg.Done()
				}()
			}

			wg.Wait()
			go func() {
				pk := createStatusHandshake(tc.portEnd)
				config := statusDialConfig{
					pk:           pk,
					expectedName: tc.expectedVersion,
					gatewayAddr:  gatewayAddr(tc.portEnd),
				}
				same, err := statusDial(config)
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

type statusListenerConfig struct {
	id     int
	addr   string
	status StatusConfig
}

func statusListen(c statusListenerConfig, errorCh chan *testError) {
	listener, err := Listen(c.addr)
	if err != nil {
		errorCh <- &testError{err, fmt.Sprintf("Can't listen to %v", c.addr)}
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				errorCh <- &testError{err, "Can't accept connection on listener"}
			}
			pk, err := c.status.StatusResponsePacket()
			if err != nil {
				errorCh <- &testError{err, "Can't create status response packet"}
			}
			go func() {
				if err := conn.WritePacket(pk); err != nil {
					errorCh <- &testError{err, "Can't write status response packet on connection"}
				}
			}()
		}
	}()
}

type statusDialConfig struct {
	pk           protocol.Packet
	expectedName string
	gatewayAddr  string
}

func statusDial(c statusDialConfig) (bool, *testError) {
	conn, err := Dial(c.gatewayAddr)
	if err != nil {
		return false, &testError{err, "Can't make a connection with gateway"}
	}
	defer conn.Close()

	if err := sendHandshake(conn, c.pk); err != nil {
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
	return c.expectedName == res.Version.Name, nil
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
	config := proxyConfigWithPortEnd(portEnd)
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
			wg := &sync.WaitGroup{}

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				config := createProxyProtocolConfig(tc.portEnd, tc.proxyproto)
				gateway := Gateway{}
				proxies := configToProxies(config)
				if err := gateway.ListenAndServe(proxies); err != nil {
					errorCh <- &testError{err, "Can't start gateway"}
				}
				wg.Done()
				gateway.KeepProcessActive()
			}(wg)

			go func() {
				if err := startProxyProtoListen(tc.portEnd, shareCh); err != nil {
					errorCh <- err
				}
			}()
			wg.Wait()
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
	listenAddr := serverAddr(portEnd)
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
	gatewayAddr := gatewayAddr(portEnd)
	conn, err := createConnWithFakeIP(gatewayAddr)
	if err != nil {
		return false, &testError{err, "Can't create connection"}
	}
	defer conn.Close()

	if err := sendHandshakePort(conn, portEnd); err != nil {
		return false, err
	}

	usedIp := getIpFromAddr(conn.LocalAddr())
	receivedIp := <-shareCh

	return validator(usedIp, receivedIp), nil
}

func routeVersionName(index int) string {
	return fmt.Sprintf("infrared.gateway-%d", index)
}

func TestRouting(t *testing.T) {
	wg := &sync.WaitGroup{}
	errorCh := make(chan *testError)

	basePort := 540
	routingConfig := make([]*ProxyConfig, 0)
	serverConfigs := make([]statusListenerConfig, 0)

	servers := []struct {
		id      int
		domain  string
		portEnd int
	}{
		{
			id:      0,
			domain:  "infrared",
			portEnd: 530,
		},
		{
			id:      9,
			domain:  "infrared",
			portEnd: 531,
		},
		{
			id:      1,
			domain:  "infrared-dash",
			portEnd: 530,
		},
		{
			id:      2,
			domain:  ".dottedInfrared.",
			portEnd: 530,
		},
	}

	tt := []struct {
		name           string
		expectedId     int
		requestDomain  string
		gatewayPortEnd int
		expectError    bool
		shouldMatch    bool
	}{
		{
			name:           "Single word domain",
			expectedId:     0,
			requestDomain:  "infrared",
			gatewayPortEnd: 530,
			expectError:    false,
			shouldMatch:    true,
		},
		{
			name:           "Single word domain but wrong id",
			expectedId:     1,
			requestDomain:  "infrared",
			gatewayPortEnd: 530,
			expectError:    false,
			shouldMatch:    false,
		},
		{
			name:           "duplicated domain but other port",
			expectedId:     9,
			requestDomain:  "infrared",
			gatewayPortEnd: 531,
			expectError:    false,
			shouldMatch:    true,
		},
		{
			name:           "Domain with a dash",
			expectedId:     1,
			requestDomain:  "infrared-dash",
			gatewayPortEnd: 530,
			expectError:    false,
			shouldMatch:    true,
		},
		{
			name:           "Domain with points at both ends",
			expectedId:     2,
			requestDomain:  ".dottedInfrared.",
			gatewayPortEnd: 530,
			expectError:    true,
			shouldMatch:    false,
		},
	}

	for i, server := range servers {
		port := basePort + i
		proxyC := &ProxyConfig{}
		serverC := statusListenerConfig{}

		serverAddr := serverAddr(port)
		proxyC.ListenTo = gatewayAddr(server.portEnd)
		proxyC.ProxyTo = serverAddr
		proxyC.DomainName = server.domain
		routingConfig = append(routingConfig, proxyC)

		serverC.id = server.id
		serverC.addr = serverAddr
		serverC.status = statusPKWithVersion(routeVersionName(server.id))
		serverConfigs = append(serverConfigs, serverC)
	}

	wg.Add(1)
	go func() {
		gateway := Gateway{}
		proxies := configsToProxies(routingConfig)
		if err := gateway.ListenAndServe(proxies); err != nil {
			errorCh <- &testError{err, "Can't start gateway"}
		}
		wg.Done()
		gateway.KeepProcessActive()
	}()

	for _, c := range serverConfigs {
		wg.Add(1)
		go func(config statusListenerConfig) {
			statusListen(config, errorCh)
			wg.Done()
		}(c)
	}

	wg.Wait()

	select {
	case err := <-errorCh:
		t.Fatalf("Unexpected Error before tests: %s\n%v", err.Message, err.Error)
	default:
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resultCh := make(chan bool)

			go func() {
				pk := serverHandshake(tc.requestDomain, tc.gatewayPortEnd)
				config := statusDialConfig{
					pk:           pk,
					expectedName: routeVersionName(tc.expectedId),
					gatewayAddr:  gatewayAddr(tc.gatewayPortEnd),
				}

				same, err := statusDial(config)
				if err != nil {
					errorCh <- err
				}
				resultCh <- same
			}()

			select {
			case err := <-errorCh:
				if !tc.expectError {
					t.Fatalf("Unexpected Error in test: %s\n%v", err.Message, err.Error)
				}
			case r := <-resultCh:
				if r != tc.shouldMatch {
					t.Fail()
				}
			}
		})
	}
}
