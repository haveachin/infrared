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

func startGatewayWithConfig(config *ProxyConfig, errCh chan<- error) {
	var proxies []*Proxy
	proxy := &Proxy{Config: config}
	proxies = append(proxies, proxy)

	startGatewayProxies(proxies, errCh)
}

func startGatewayProxies(proxies []*Proxy, errCh chan<- error) {
	go func() {
		gateway := Gateway{}
		if err := gateway.ListenAndServe(proxies); err != nil {
			errCh <- err
			return
		}
	}()
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
			cDial := make(chan error)
			cListen := make(chan error)
			cGateway := make(chan error)
			cResult := make(chan bool)

			config := createBasicProxyConfig(tc.portEnd)
			config.OnlineStatus = tc.onlineStatus
			config.OfflineStatus = tc.offlineStatus

			startGatewayWithConfig(config, cGateway)

			if tc.activeServer {
				startStatusListen(tc.portEnd, cListen)
			}

			startStatusDial(tc.portEnd, cDial, cResult, tc.expectedVersion)

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

func startStatusListen(portEnd int, errCh chan<- error) {
	go func() {
		listenAddr := portToAddr(listenPort(portEnd))
		listener, err := Listen(listenAddr)
		// if err != nil {
		// 	errCh <- err
		// 	return
		// }
		defer listener.Close()

		conn, err := listener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		pk, err := statusConfig.StatusResponsePacket()
		if err != nil {
			errCh <- err
			return
		}
		conn.WritePacket(pk)
	}()
}

func startStatusDial(portEnd int, errCh chan<- error, resultCh chan<- bool, expectedName string) {
	go func() {
		time.Sleep(dialWait) // Startup time for gateway
		gatewayPort := gatewayPort(portEnd)
		gatewayAddr := portToAddr(gatewayPort)
		conn, _ := Dial(gatewayAddr)
		defer conn.Close()

		hsPk := createStatusHankshake(portEnd)
		statusPk := status.ServerBoundRequest{}.Marshal()

		if err := conn.WritePacket(hsPk); err != nil {
			errCh <- err
			return
		}

		if err := conn.WritePacket(statusPk); err != nil {
			errCh <- err
			return
		}

		receivedPk, err := conn.ReadPacket()
		if err != nil {
			errCh <- err
			return
		}

		response, err := status.UnmarshalClientBoundResponse(receivedPk)
		if err != nil {
			errCh <- err
			return
		}

		res := &status.ResponseJSON{}
		json.Unmarshal([]byte(response.JSONResponse), &res)
		resultCh <- expectedName == res.Version.Name
	}()
}
