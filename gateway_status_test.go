package infrared

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
)

var dialWait time.Duration = time.Duration(5 * time.Millisecond)

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

func TestConnWithGatewayInBetween(t *testing.T) {
	portEnd := 569
	listenAddr := portToAddr(listenPort(portEnd))
	gatewayAddr := portToAddr(gatewayPort(portEnd))

	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)

	config := &ProxyConfig{DomainName: serverAddr, ListenTo: gatewayAddr, ProxyTo: listenAddr}
	startGatewayWithConfig(config, cGateway)

	go func() {
		listener, err := Listen(listenAddr)
		if err != nil {
			cListen <- err
			return
		}
		defer listener.Close()

		conn, err := listener.Accept()
		if err != nil {
			cListen <- err
			return
		}
		defer conn.Close()

		pk, _ := conn.PeekPacket()
		hs, _ := handshaking.UnmarshalServerBoundHandshake(pk)
		givenServerAddr := hs.ParseServerAddress()

		cResult <- givenServerAddr == serverAddr
	}()

	startStatusDial(portEnd, cDial, cResult, statusResponse.Version)

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

var statusSamples []status.PlayerSampleJSON = make([]status.PlayerSampleJSON, 0)
var statusResponse status.ResponseJSON = status.ResponseJSON{
	Version: status.VersionJSON{
		Name:     "Infrared-test-online",
		Protocol: 754,
	},
	Players: status.PlayersJSON{
		Max:    20,
		Online: 0,
		Sample: statusSamples,
	},
	Description: status.DescriptionJSON{
		Text: "Server MOTD",
	},
}

var configStatusOnlineWithoutConfig *ProxyConfig = createBasicProxyConfig(570)
var configStatusOnlineWithConfig *ProxyConfig = createBasicProxyConfig(571)

func TestStatusRequest(t *testing.T) {
	configStatusOnlineWithConfig.OnlineStatus = StatusConfig{
		VersionName:    "Infrared 1.16.5",
		ProtocolNumber: 754,
		MaxPlayers:     20,
		MOTD:           "Powered by Infrared",
	}

	tt := []struct {
		name            string
		portEnd         int
		config          *ProxyConfig
		expectedVersion status.VersionJSON
	}{
		{
			name:            "StatusOnlineWithoutConfig",
			portEnd:         570,
			config:          configStatusOnlineWithoutConfig,
			expectedVersion: statusResponse.Version,
		},
		{
			name:    "StatusOnlineWithConfig",
			portEnd: 571,
			config:  configStatusOnlineWithConfig,
			expectedVersion: status.VersionJSON{
				Name:     "Infrared 1.16.5",
				Protocol: 754,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cDial := make(chan error)
			cListen := make(chan error)
			cGateway := make(chan error)
			cResult := make(chan bool)

			startGatewayWithConfig(tc.config, cGateway)

			startStatusListen(tc.portEnd, cListen)
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

var samples []PlayerSample = make([]PlayerSample, 0)
var statusConfig StatusConfig = StatusConfig{VersionName: "Infrared-test-online", ProtocolNumber: 754,
	MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}

func startStatusListen(portEnd int, errCh chan<- error) {
	go func() {
		listenAddr := portToAddr(listenPort(portEnd))
		listener, err := Listen(listenAddr)
		if err != nil {
			errCh <- err
			return
		}
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

func startStatusDial(portEnd int, errCh chan<- error, resultCh chan<- bool, expectedVersion status.VersionJSON) {
	go func() {
		time.Sleep(dialWait) // Startup time for gateway
		gatewayPort := gatewayPort(portEnd)
		gatewayAddr := portToAddr(gatewayPort)
		conn, err := Dial(gatewayAddr)
		if err != nil {
			errCh <- err
			return
		}
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
		resultCh <- expectedVersion == res.Version
	}()
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
