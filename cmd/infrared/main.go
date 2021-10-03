package main

import (
	"fmt"
	"log"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/haveachin/infrared"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var logger logr.Logger

func init() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to init logger; err: %s", err))
	}
	logger = zapr.NewLogger(zapLog)
}

func main() {
	logger.Info(viper.GetString("api.bind"))
	log.Println(loadGateways())
	log.Println(loadServers())
	log.Println(loadWebhooks())

	cpnChan := make(chan infrared.ProcessingConn)
	srvChan := make(chan infrared.ProcessingConn)
	poolChan := make(chan infrared.ProcessedConn)

	gw := infrared.Gateway{
		Binds:                []string{":25565"},
		ReceiveProxyProtocol: false,
		ReceiveRealIP:        false,
		ServerIDs:            []string{"test"},
		Log:                  logger,
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
		Log:        logger,
	}

	srvGw := infrared.ServerGateway{
		Servers: []infrared.Server{srvTest},
		Log:     logger,
	}

	for i := 0; i < 10; i++ {
		cpn := infrared.CPN{
			Log: logger,
		}
		go cpn.Start(cpnChan, srvChan)
	}

	pool := infrared.ConnPool{
		Log: logger,
	}

	go pool.Start(poolChan)
	go srvGw.Start(srvChan, poolChan)
	gw.Start(cpnChan)
}
