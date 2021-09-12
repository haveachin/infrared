package main

import (
	"log"

	"github.com/haveachin/infrared"
)

// Amount of Connection Processing Nodes (CPN)
const cpnCount = 10

var configFile = "./config.yml"

func main() {
	log.Println(configFile)

	cpnChan := make(chan infrared.ProcessingConn)
	srvChan := make(chan infrared.ProcessingConn)
	poolChan := make(chan infrared.ProcessedConn)

	gw := infrared.Gateway{
		Binds:                []string{":25565"},
		ReceiveProxyProtocol: false,
		ReceiveRealIP:        false,
		ServerIDs:            []string{"test"},
	}

	srvTest := infrared.Server{
		ID:                "test",
		Domains:           []string{"localhost", "127.0.0.1"},
		Address:           "localhost:25566",
		SendProxyProtocol: false,
		SendRealIP:        false,
		DisconnectMessage: "Hey §4§l{{username}}§r, your address is {{remoteAddress}}",
		OnlineStatus: infrared.StatusResponse{
			MOTD: "Fuck you {{username}}",
		},
		OfflineStatus: infrared.StatusResponse{
			VersionName:    "Infrared v2 1.17.1",
			ProtocolNumber: 756,
			MaxPlayers:     20,
			PlayersOnline:  0,
			PlayerSamples:  nil,
			IconPath:       "icon.png",
			MOTD:           "§cInfrared v2 Proxy\n§4§l{{domain}}§r is an invalid host",
		},
		WebhookIDs: []string{},
	}

	srvGw := infrared.ServerGateway{
		Servers: []infrared.Server{srvTest},
	}

	for i := 0; i < cpnCount; i++ {
		cpn := infrared.CPN{}
		go cpn.Start(cpnChan, srvChan)
	}

	pool := infrared.ConnPool{}

	go pool.Start(poolChan)
	go srvGw.Start(srvChan, poolChan)
	gw.Start(cpnChan)
}

/* func dos(addr string) {
	for {
		rc, _ := net.Dial("tcp", addr)
		rc.Write([]byte{16, 0, 244, 5, 9, 108, 111, 99, 97, 108, 104, 111, 115, 116, 99, 221, 1})
		rc.Write([]byte{1, 0})
		rc.Close()
	}
} */
