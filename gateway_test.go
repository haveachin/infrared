package infrared

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

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
		NextState:       1, //one means status
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

	factory := createTestFactory()

	for _, c := range config {
		proxy := &Proxy{Config: c}
		proxy.ServerFactory = factory
		proxies = append(proxies, proxy)
	}
	return proxies
}

func createTestFactory() func(p *Proxy) MCServer {
	return func(p *Proxy) MCServer {
		timeout := p.Timeout()
		serverAddr := p.ProxyTo()
		return &e2eTestServer{
			ServerAddr: serverAddr,
			Timeout:    timeout,
		}
	}
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

func statusReponseToStruct(pk protocol.Packet) (status.ResponseJSON, error) {
	response, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return status.ResponseJSON{}, err
	}

	res := &status.ResponseJSON{}
	json.Unmarshal([]byte(response.JSONResponse), &res)
	return *res, nil
}

type statusDialConfig struct {
	conn                    *Conn
	pk                      protocol.Packet
	gatewayAddr             string
	dialerPort              int
	useProxyProtocol        bool
	sendProxyProtocolHeader bool
	sendEndPing             bool
	sendNonPacketData       bool
}

func statusDial(c statusDialConfig) (protocol.Packet, *testError) {
	var conn Conn
	var err error
	if c.conn != nil {
		conn = *c.conn
	} else if c.useProxyProtocol {
		conn, err = createConnWithFakeIP(c.dialerPort, c.gatewayAddr)
	} else {
		conn, err = Dial(c.gatewayAddr)
	}

	if err != nil {
		return protocol.Packet{}, &testError{err, "Can't make a connection with gateway"}
	}
	defer conn.Close()

	if c.sendProxyProtocolHeader {
		if err := sendProxyProtocolHeader(conn); err != nil {
			return protocol.Packet{}, err
		}
	}

	if c.sendNonPacketData {
		//           	ID | ProtoVer. | Server Address                                                   		|Serv. Port | Nxt State
		// data := []byte{0x00, 0xC2, 0x04, 0x0B, 0x65, 0x78, 0x61, 0x6D, 0x70, 0x6C, 0x65, 0x2E, 0x63, 0x6F, 0x6D, 0x05, 0x39, 0x01}
		data := []byte{0, 1, 2, 3, 0xFF}
		conn.Write(data)
		return protocol.Packet{}, nil
	}

	if err := sendHandshake(conn, c.pk); err != nil {
		return protocol.Packet{}, err
	}

	statusPk := status.ServerBoundRequest{}.Marshal()
	if err := conn.WritePacket(statusPk); err != nil {
		return protocol.Packet{}, &testError{err, "Can't write status request packet"}
	}

	receivedPk, err := conn.ReadPacket()
	if err != nil {
		return protocol.Packet{}, &testError{err, "Can't read status reponse packet"}
	}

	if c.sendEndPing {
		pingPk := status.ServerBoundRequest{}.Marshal()
		if err := conn.WritePacket(pingPk); err != nil {
			return receivedPk, &testError{err, "couldnt send packet for ping to server"}
		}
		conn.ReadPacket()
	}

	return receivedPk, nil

}

func statusDialGetVersionName(c statusDialConfig) (string, *testError) {
	pk, err := statusDial(c)
	if err != nil {
		return "", err
	}
	res, err2 := statusReponseToStruct(pk)
	if err2 != nil {
		return "", &testError{err2, "Couldn't convert response to ResponseJSON struct"}
	}
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

type e2eTestServer struct {
	connection Conn
	ServerAddr string
	Timeout    time.Duration
}

func (s *e2eTestServer) CanConnect() bool {
	var err error
	s.connection, err = DialTimeout(s.ServerAddr, s.Timeout)
	return err == nil
}

func (s *e2eTestServer) Connection() (Conn, error) {
	return s.connection, nil
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
				pk := statusHandshakePort(tc.portEnd)
				config := statusDialConfig{
					pk:          pk,
					gatewayAddr: gatewayAddr(tc.portEnd),
				}
				receivedVersion, err := statusDialGetVersionName(config)
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
			name:        "ProxyProtocolOn",
			proxyproto:  true,
			portEnd:     581,
			shouldMatch: true,
			expectingIp: "127.0.0.1",
		},
		{
			name:        "ProxyProtocolOff",
			proxyproto:  false,
			portEnd:     582,
			shouldMatch: true,
			expectingIp: "127.0.0.1",
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
				gateway := Gateway{}
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

				// if tc.proxyproto {

				// 	dialer := &net.Dialer{
				// 		LocalAddr: &net.TCPAddr{
				// 			IP:   net.ParseIP("127.0.10.1"),
				// 			Port: dialerPort(tc.portEnd),
				// 		},
				// 	}
				// 	netConn, _ := dialer.Dial("tcp", gatewayAddr(tc.portEnd))
				// 	config.conn = createTestConn(netConn)
				// }

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

				receivedVersion, err := statusDialGetVersionName(config)
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

func createTestConn(conn net.Conn) Conn {
	return wrapConn(conn)
}

type alwaysOnlineServer struct {
	conn Conn
}

func (s *alwaysOnlineServer) CanConnect() bool {
	return true
}

func (s *alwaysOnlineServer) Connection() (Conn, error) {
	return s.conn, nil
}

type alwaysOfflineServer struct {
}

func (s *alwaysOfflineServer) CanConnect() bool {
	return false
}

func (s *alwaysOfflineServer) Connection() (Conn, error) {
	return nil, nil
}

func TestPacketHandling(t *testing.T) {
	//Bc its necessary for every proxy to have this
	domain := serverDomain
	proxyTo := ":25560"
	tt := []struct {
		name                    string
		expectError             error
		sendProxyProtocolHeader bool
		sendCorruptedPacket     bool
		sendNonPacketData       bool
		isProxyProtocolServer   bool
		onlineServer            bool
	}{
		{
			name: "offline server without errors",
		},
		{
			name:                "send incorrect packet",
			sendCorruptedPacket: true,
			expectError:         proxyproto.ErrNoProxyProtocol,
		},
		{
			name:                    "send proxyprotocol with normal packet",
			sendProxyProtocolHeader: true,
		},
		{
			name:                    "send proxyprotocol with incorrect packet",
			sendProxyProtocolHeader: true,
			sendCorruptedPacket:     true,
			expectError:             ErrCantUnMarshalPK,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			cServer := createTestConn(c1)
			cClient := createTestConn(c2)

			proxyConfig := &ProxyConfig{DomainName: domain, ProxyTo: proxyTo}

			if tc.isProxyProtocolServer {
				proxyConfig.ProxyProtocol = tc.isProxyProtocolServer
			}

			factory := func(p *Proxy) MCServer {
				return &alwaysOfflineServer{}
			}
			if tc.onlineServer {
				factory = func(p *Proxy) MCServer {
					return &alwaysOnlineServer{nil}
				}
			}

			proxy := &Proxy{Config: proxyConfig}
			proxy.ServerFactory = factory

			gateway := &Gateway{}
			gateway.proxies.Store(proxy.UID(), proxy)

			go func(c Conn) {
				pk := serverHandshake(domain, 25565)

				if tc.sendCorruptedPacket {
					corruptedData := pk.Data[1:12]
					pk.Data = corruptedData
					t.Log(pk)
				}

				dialConfig := statusDialConfig{
					conn:                    &c,
					pk:                      pk,
					sendProxyProtocolHeader: tc.sendProxyProtocolHeader,
					sendEndPing:             true,
				}

				t.Log(dialConfig)
				_, err := statusDial(dialConfig)
				if err != nil {
					fmt.Println(err)
				}
			}(cClient)

			if err := gateway.serve(cServer, ""); err != nil {
				if tc.expectError == nil {
					t.Errorf("didnt expect an error but got %v", err)
				}
				if errors.Is(err, tc.expectError) {
					return
				}
				t.Errorf("got different error than expected, got: %v\n expected: %v", err, tc.expectError)
				return
			}
			if tc.expectError != nil {
				t.Errorf("Expected an error but didnt got any, expected error: %v", tc.expectError)
			}
		})
	}
}
