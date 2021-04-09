package infrared

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
)

var serverAddr string = "infrared.gateway"
var dialWait time.Duration = time.Duration(5 * time.Millisecond) // Startup time for gateway

func listenPort(portEnd int) int {
	return 20000 + portEnd
}

func gatewayPort(portEnd int) int {
	return 30000 + portEnd
}

func portToAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

func createBasicProxyConfig(portEnd int) *ProxyConfig {
	listenAddr := portToAddr(listenPort(portEnd))
	gatewayAddr := portToAddr(gatewayPort(portEnd))

	return &ProxyConfig{
		DomainName: serverAddr,
		ListenTo:   gatewayAddr,
		ProxyTo:    listenAddr,
	}
}

func createStatusHankshake(portEnd int) protocol.Packet {
	gatewayPort := gatewayPort(portEnd)
	hs := handshaking.ServerBoundHandshake{
		ProtocolVersion: 574,
		ServerAddress:   protocol.String(serverAddr),
		ServerPort:      protocol.UnsignedShort(gatewayPort),
		NextState:       1,
	}
	return hs.Marshal()
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

var serverVersionName string = "Infrared-test-online"
var samples []PlayerSample = make([]PlayerSample, 0)
var statusConfig StatusConfig = StatusConfig{VersionName: serverVersionName, ProtocolNumber: 754,
	MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}

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

type testError struct {
	Error   error
	Message string
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
			onlineStatus:    StatusConfig{},
			offlineStatus:   StatusConfig{},
			activeServer:    true,
			expectedVersion: serverVersionName,
		},
		{
			name:            "ServerOfflineWithoutConfig",
			portEnd:         571,
			onlineStatus:    StatusConfig{},
			offlineStatus:   StatusConfig{},
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
				same, err := startStatusDial(tc.portEnd, tc.expectedVersion)
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

func startStatusListen(portEnd int) *testError {
	listenAddr := portToAddr(listenPort(portEnd))
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

func startStatusDial(portEnd int, expectedName string) (bool, *testError) {
	time.Sleep(dialWait) // Startup time for gateway
	gatewayPort := gatewayPort(portEnd)
	gatewayAddr := portToAddr(gatewayPort)
	conn, err := Dial(gatewayAddr)
	if err != nil {
		return false, &testError{err, "Can't make a connection with gateway"}
	}
	defer conn.Close()

	hsPk := createStatusHankshake(portEnd)
	statusPk := status.ServerBoundRequest{}.Marshal()

	if err := conn.WritePacket(hsPk); err != nil {
		return false, &testError{err, "Can't write handshake"}
	}
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
