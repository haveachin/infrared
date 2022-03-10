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

func dialerPort(portEnd int) int {
	return 10000 + portEnd
}

func portToAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

func routeVersionName(index int) string {
	return fmt.Sprintf("infrared.gateway-%d", index)
}

func getIpFromAddr(addr net.Addr) string {
	return strings.Split(addr.String(), ":")[0]
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

func createProxyProtocolConfig(portEnd int, proxyproto bool) *ProxyConfig {
	config := proxyConfigWithPortEnd(portEnd)
	config.ProxyProtocol = proxyproto
	return config
}

func statusHandshakePort(portEnd int) protocol.Packet {
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

func sendProxyProtocolHeader(rconn Conn) *testError {
	header := createProxyProtocolHeader()
	if _, err := header.WriteTo(rconn); err != nil {
		return &testError{err, "Can't write proxy protocol header"}
	}
	return nil
}

var serverVersionName = "Infrared-test-online"

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
	pk                      protocol.Packet
	gatewayAddr             string
	dialerPort              int
	sendProxyProtocolHeader bool
	useProxyProtocol        bool
}

func statusDial(c statusDialConfig) (string, *testError) {
	var conn Conn
	var err error
	if c.useProxyProtocol {
		conn, err = createConnWithFakeIP(c.dialerPort, c.gatewayAddr)
	} else {
		conn, err = Dialer{}.Dial(c.gatewayAddr)
	}

	if err != nil {
		return "", &testError{err, "Can't make a connection with gateway"}
	}
	defer conn.Close()

	if c.sendProxyProtocolHeader {
		if err := sendProxyProtocolHeader(conn); err != nil {
			return "", err
		}
	}

	if err := sendHandshake(conn, c.pk); err != nil {
		return "", err
	}

	statusPk := status.ServerBoundRequest{}.Marshal()
	if err := conn.WritePacket(statusPk); err != nil {
		return "", &testError{err, "Can't write status request packet"}
	}

	receivedPk, err := conn.ReadPacket()
	if err != nil {
		return "", &testError{err, "Can't read status reponse packet"}
	}

	response, err := status.UnmarshalClientBoundResponse(receivedPk)
	if err != nil {
		return "", &testError{err, "Can't unmarshal status reponse packet"}
	}

	res := &status.ResponseJSON{}
	json.Unmarshal([]byte(response.JSONResponse), &res)
	return res.Version.Name, nil
}

func createConnWithFakeIP(dialerPort int, gatewayAddr string) (Conn, error) {
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: dialerPort,
		},
	}
	netConn, err := dialer.Dial("tcp", gatewayAddr)
	if err != nil {
		return nil, err
	}
	return wrapConn(netConn), nil
}

func createProxyProtocolHeader() proxyproto.Header {
	return proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("109.226.143.210"),
			Port: 0,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("210.223.216.109"),
			Port: 0,
		},
	}
}

func proxyProtoListen(portEnd int) (string, *testError) {
	listenAddr := serverAddr(portEnd)
	listener, err := Listen(listenAddr)
	if err != nil {
		return "", &testError{err, fmt.Sprintf("Can't listen to %v", listenAddr)}
	}
	defer listener.Close()

	proxyListener := &proxyproto.Listener{Listener: listener.Listener}
	defer proxyListener.Close()

	conn, err := proxyListener.Accept()
	if err != nil {
		return "", &testError{err, "Can't accept connection on listener"}
	}
	defer conn.Close()
	return getIpFromAddr(conn.RemoteAddr()), nil
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
				serverCfg := statusListenerConfig{}
				serverCfg.status = statusPKWithVersion(serverVersionName)
				serverCfg.addr = serverAddr(tc.portEnd)
				go func() {
					statusListen(serverCfg, errorCh)
					wg.Done()
				}()
			}

			wg.Wait()
			go func() {
				pk := statusHandshakePort(tc.portEnd)
				config := statusDialConfig{
					pk:          pk,
					gatewayAddr: gatewayAddr(tc.portEnd),
					dialerPort:  dialerPort(tc.portEnd),
				}
				receivedVersion, err := statusDial(config)
				if err != nil {
					errorCh <- err
					return
				}

				resultCh <- receivedVersion == tc.expectedVersion
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
		name              string
		proxyproto        bool
		receiveProxyproto bool
		portEnd           int
		shouldMatch       bool
		expectingIp       string
	}{
		{
			name:              "ProxyProtocolOn",
			proxyproto:        true,
			receiveProxyproto: false,
			portEnd:           581,
			shouldMatch:       true,
			expectingIp:       "127.0.0.1",
		},
		{
			name:              "ProxyProtocolOff",
			proxyproto:        false,
			receiveProxyproto: false,
			portEnd:           582,
			shouldMatch:       true,
			expectingIp:       "127.0.0.1",
		},
		{
			name:              "ProxyProtocol Receive",
			proxyproto:        true,
			receiveProxyproto: true,
			portEnd:           583,
			shouldMatch:       true,
			expectingIp:       "109.226.143.210",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errorCh := make(chan *testError)
			resultCh := make(chan bool)
			wg := &sync.WaitGroup{}

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				config := createProxyProtocolConfig(tc.portEnd, tc.proxyproto)
				gateway := Gateway{
					ReceiveProxyProtocol: tc.receiveProxyproto,
				}
				proxies := configToProxies(config)
				if err := gateway.ListenAndServe(proxies); err != nil {
					errorCh <- &testError{err, "Can't start gateway"}
				}
				wg.Done()
				gateway.KeepProcessActive()
			}(wg)

			go func() {
				ip, err := proxyProtoListen(tc.portEnd)
				if err != nil {
					errorCh <- err
					return
				}
				resultCh <- ip == tc.expectingIp
			}()
			wg.Wait()
			go func() {

				pk := statusHandshakePort(tc.portEnd)
				config := statusDialConfig{
					pk:                      pk,
					gatewayAddr:             gatewayAddr(tc.portEnd),
					dialerPort:              dialerPort(tc.portEnd),
					useProxyProtocol:        tc.proxyproto,
					sendProxyProtocolHeader: tc.receiveProxyproto,
				}

				_, err := statusDial(config)
				if err != nil {
					errorCh <- err
				}
			}()

			select {
			case err := <-errorCh:
				t.Fatalf("Unexpected Error in test: %s\n%v", err.Message, err.Error)
			case r := <-resultCh:
				if r != tc.shouldMatch {
					t.Errorf("got: %v; want: %v", r, tc.shouldMatch)
				}
			}

		})
	}
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
		name          string
		expectedId    int
		requestDomain string
		portEnd       int
		expectError   bool
		shouldMatch   bool
	}{
		{
			name:          "Single word domain",
			expectedId:    0,
			requestDomain: "infrared",
			portEnd:       530,
			expectError:   false,
			shouldMatch:   true,
		},
		{
			name:          "Single word domain but wrong id",
			expectedId:    1,
			requestDomain: "infrared",
			portEnd:       530,
			expectError:   false,
			shouldMatch:   false,
		},
		{
			name:          "duplicated domain but other port",
			expectedId:    9,
			requestDomain: "infrared",
			portEnd:       531,
			expectError:   false,
			shouldMatch:   true,
		},
		{
			name:          "Domain with a dash",
			expectedId:    1,
			requestDomain: "infrared-dash",
			portEnd:       530,
			expectError:   false,
			shouldMatch:   true,
		},
		{
			name:          "Domain with points at both ends",
			expectedId:    2,
			requestDomain: ".dottedInfrared.",
			portEnd:       530,
			expectError:   true,
			shouldMatch:   false,
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
				expectedName := routeVersionName(tc.expectedId)
				pk := serverHandshake(tc.requestDomain, tc.portEnd)
				config := statusDialConfig{
					pk:          pk,
					gatewayAddr: gatewayAddr(tc.portEnd),
					dialerPort:  dialerPort(tc.portEnd),
				}

				receivedVersion, err := statusDial(config)
				if err != nil {
					errorCh <- err
					return
				}
				resultCh <- receivedVersion == expectedName
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

func TestProxyBind(t *testing.T) {
	// TODO: Figure out a way to test this
}
