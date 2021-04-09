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

func TestConnWithGatewayInBetween(t *testing.T) {
	serverAddr := "infrared.gateway"
	listenPort := ":26569"
	gatewayPort := ":25569"
	isHandshakePk := handshaking.ServerBoundHandshake{
		ProtocolVersion: 578,
		ServerAddress:   "infrared.gateway",
		ServerPort:      25569,
		NextState:       1,
	}

	pk := isHandshakePk.Marshal()
	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)

	go func() {
		config := &ProxyConfig{DomainName: serverAddr, ListenTo: gatewayPort, ProxyTo: listenPort}
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
		listener, err := Listen(listenPort)
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

	go func() {
		time.Sleep(time.Second * 1) // Startup time for gateway
		conn, err := Dial(gatewayPort)
		if err != nil {
			cDial <- err
			return
		}
		defer conn.Close()

		if err := conn.WritePacket(pk); err != nil {
			cDial <- err
			return
		}
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

func TestStatusRequest_Online_WithoutConfig(t *testing.T) {
	serverAddr := "infrared.gateway"
	listenPort := ":26570"
	gatewayPort := ":25570"

	samples := make([]PlayerSample, 0)
	statusConf := &StatusConfig{VersionName: "Infrared-test-online", ProtocolNumber: 754,
		MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}

	expectedSamples := make([]status.PlayerSampleJSON, 0)
	expectedStatus := &status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     "Infrared-test-online",
			Protocol: 754,
		},
		Players: status.PlayersJSON{
			Max:    20,
			Online: 0,
			Sample: expectedSamples,
		},
		Description: status.DescriptionJSON{
			Text: "Server MOTD",
		},
	}

	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)

	go func() {
		config := &ProxyConfig{DomainName: serverAddr, ListenTo: gatewayPort, ProxyTo: listenPort}
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
		listener, err := Listen(listenPort)
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

		pk, err := statusConf.StatusResponsePacket()

		if err != nil {
			cListen <- err
			return
		}

		conn.WritePacket(pk)
	}()

	go func() {
		time.Sleep(time.Second * 1) // Startup time for gateway
		conn, err := Dial(gatewayPort)
		if err != nil {
			cDial <- err
			return
		}
		defer conn.Close()

		hs := handshaking.ServerBoundHandshake{
			ProtocolVersion: 754,
			ServerAddress:   "infrared.gateway",
			ServerPort:      25570,
			NextState:       1,
		}

		hsPk := hs.Marshal()
		statusPk := status.ServerBoundRequest{}.Marshal()

		if err := conn.WritePacket(hsPk); err != nil {
			cDial <- err
			return
		}

		if err := conn.WritePacket(statusPk); err != nil {
			cDial <- err
			return
		}

		receivedPk, err := conn.ReadPacket()
		if err != nil {
			cDial <- err
			return
		}

		response, err := status.UnmarshalClientBoundResponse(receivedPk)
		if err != nil {
			cDial <- err
			return
		}

		res := &status.ResponseJSON{}
		json.Unmarshal([]byte(response.JSONResponse), &res)

		cResult <- expectedStatus.Version.Name == res.Version.Name
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

func TestStatusRequest_Online_WithConfig(t *testing.T) {
	serverAddr := "infrared.gateway"
	listenAddr := ":26571"
	gatewayPort := 25571
	gatewayAddr := fmt.Sprintf(":%d", gatewayPort)

	samples := make([]PlayerSample, 0)
	statusConf := &StatusConfig{VersionName: "Infrared-test-online", ProtocolNumber: 754,
		MaxPlayers: 20, PlayersOnline: 0, PlayerSamples: samples, MOTD: "Server MOTD"}

	expectedVersion := &status.VersionJSON{
		Name:     "Infrared 1.16.5",
		Protocol: 754,
	}

	cDial := make(chan error)
	cListen := make(chan error)
	cGateway := make(chan error)
	cResult := make(chan bool)

	go func() {
		config := &ProxyConfig{DomainName: serverAddr,
			ListenTo: gatewayAddr,
			ProxyTo:  listenAddr,
			OnlineStatus: StatusConfig{
				VersionName:    "Infrared 1.16.5",
				ProtocolNumber: 754,
				MaxPlayers:     20,
				MOTD:           "Powered by Infrared",
			}}
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

		conn, err := listener.Accept()
		if err != nil {
			cListen <- err
			return
		}
		defer conn.Close()

		statusPk, err := statusConf.StatusResponsePacket()

		if err != nil {
			cListen <- err
			return
		}

		conn.WritePacket(statusPk)
	}()

	go func() {
		time.Sleep(time.Second * 1) // Startup time for gateway
		conn, err := Dial(gatewayAddr)
		if err != nil {
			cDial <- err
			return
		}
		defer conn.Close()

		hs := handshaking.ServerBoundHandshake{
			ProtocolVersion: 754,
			ServerAddress:   protocol.String(serverAddr),
			ServerPort:      protocol.UnsignedShort(gatewayPort),
			NextState:       1,
		}

		hsPk := hs.Marshal()
		statusPk := status.ServerBoundRequest{}.Marshal()

		if err := conn.WritePacket(hsPk); err != nil {
			cDial <- err
			return
		}

		if err := conn.WritePacket(statusPk); err != nil {
			cDial <- err
			return
		}

		receivedPk, err := conn.ReadPacket()
		if err != nil {
			cDial <- err
			return
		}

		response, err := status.UnmarshalClientBoundResponse(receivedPk)
		if err != nil {
			cDial <- err
			return
		}

		res := &status.ResponseJSON{}
		json.Unmarshal([]byte(response.JSONResponse), &res)
		cResult <- expectedVersion.Name == res.Version.Name
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
