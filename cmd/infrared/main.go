package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/haveachin/infrared"
	"go.uber.org/zap"
)

var logger logr.Logger

func init() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to init logger; err: %s", err)
	}
	logger = zapr.NewLogger(zapLog)
}

func main() {
	cpnChan := make(chan infrared.ProcessingConn, 10)
	srvChan := make(chan infrared.ProcessingConn)
	poolChan := make(chan infrared.ProcessedConn)

	startGateways(cpnChan)
	startCPNs(cpnChan, srvChan)
	go startServers(srvChan, poolChan)
	startConnPool(poolChan)

	logger.Info("done")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func startGateways(cpnChan chan<- infrared.ProcessingConn) {
	gateways, err := loadGateways()
	if err != nil {
		logger.Error(err, "loading gateways")
		return
	}

	for _, gw := range gateways {
		gw.Log = logger
		go gw.Start(cpnChan)
	}
}

func startCPNs(cpnChan <-chan infrared.ProcessingConn, srvChan chan<- infrared.ProcessingConn) {
	cpns, err := loadCPNs()
	if err != nil {
		logger.Error(err, "loading CPNs")
		return
	}

	for _, cpn := range cpns {
		cpn.Log = logger
		go cpn.Start(cpnChan, srvChan)
	}
}

func startServers(srvChan <-chan infrared.ProcessingConn, poolChan chan<- infrared.ProcessedConn) error {
	servers, err := loadServers()
	if err != nil {
		return err
	}

	for _, srv := range servers {
		srv.Log = logger
	}

	srvGw := infrared.ServerGateway{
		Servers: servers,
		Log:     logger,
	}

	if err := srvGw.Start(srvChan, poolChan); err != nil {
		return err
	}

	return nil
}

func startConnPool(poolChan <-chan infrared.ProcessedConn) {
	pool := infrared.ConnPool{
		Log: logger,
	}
	go pool.Start(poolChan)
}
