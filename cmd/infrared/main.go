package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/gateway"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/server"
)

const (
	envPrefix     = "INFRARED_"
	envConfigPath = envPrefix + "CONFIG_PATH"
)

const (
	clfConfigPath = "config-path"
)

var (
	configPath = "./configs"
)

func envBool(name string, value bool) bool {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	envBool, err := strconv.ParseBool(envString)
	if err != nil {
		return value
	}

	return envBool
}

func envString(name string, value string) string {
	envString := os.Getenv(name)
	if envString == "" {
		return value
	}

	return envString
}

func initEnv() {
	configPath = envString(envConfigPath, configPath)
}

func initFlags() {
	flag.StringVar(&configPath, clfConfigPath, configPath, "path of all proxy configs")
	flag.Parse()
}

func init() {
	initEnv()
	initFlags()
}

func main() {
	log.Println("Loading proxy configs")

	cfgs, err := infrared.LoadProxyConfigsFromPath(configPath, false)
	if err != nil {
		log.Printf("Failed loading proxy configs from %s; error: %s", configPath, err)
		return
	}

	//Designed for single config needs to be rewritten
	for _, config := range cfgs {
		gatewayCh := make(chan connection.HSConnection)
		serverCh := make(chan connection.HSConnection)

		//Listener
		outerListener := gateway.CreateBasicOuterListener(config.ListenTo)
		l := gateway.BasicListener{OutListener: outerListener, ConnCh: gatewayCh}

		go func() {
			l.Listen()
		}()

		//Gateway
		serverData := gateway.ServerData{ConnCh: serverCh}
		serverStore := &gateway.SingleServerStore{Server: serverData}

		gw := gateway.CreateBasicGatewayWithStore(serverStore, gatewayCh)
		go func() {
			gw.Start()
		}()

		//Server
		connFactory := func(addr string) (connection.ServerConnection, error) {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			conn := connection.CreateBasicConnection(c)
			return connection.CreateBasicServerConn(conn, protocol.Packet{}), nil
		}

		onlineStatus := protocol.Packet{}  //config.OnlineStatus.StatusResponsePacket()
		offlineStatus := protocol.Packet{} // config.OfflineStatus.StatusResponsePacket()

		mcServer := &server.MCServer{
			Config:              *config,
			ConnFactory:         connFactory,
			OnlineConfigStatus:  onlineStatus,
			OfflineConfigStatus: offlineStatus,
			ConnCh:              serverCh,
		}

		go func() {
			mcServer.Start()
		}()
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()

}

// func main() {
// 	log.Println("Loading proxy configs")

// 	cfgs, err := infrared.LoadProxyConfigsFromPath(configPath, false)
// 	if err != nil {
// 		log.Printf("Failed loading proxy configs from %s; error: %s", configPath, err)
// 		return
// 	}

// 	var proxies []*infrared.Proxy
// 	for _, cfg := range cfgs {
// 		proxies = append(proxies, &infrared.Proxy{
// 			Config: cfg,
// 		})
// 	}

// 	outCfgs := make(chan *infrared.ProxyConfig)
// 	go func() {
// 		if err := infrared.WatchProxyConfigFolder(configPath, outCfgs); err != nil {
// 			log.Println("Failed watching config folder; error:", err)
// 			log.Println("SYSTEM FAILURE: CONFIG WATCHER FAILED")
// 		}
// 	}()

// 	gateway := infrared.Gateway{}
// 	go func() {
// 		for {
// 			cfg, ok := <-outCfgs
// 			if !ok {
// 				return
// 			}

// 			proxy := &infrared.Proxy{Config: cfg}
// 			proxy.ServerFactory = func (p *infrared.Proxy) infrared.MCServer {
// 				timeout := p.Timeout()
// 				serverAddr := p.ProxyTo()
// 				return &infrared.BasicServer{
// 					ServerAddr: serverAddr,
// 					Timeout: timeout,
// 				}
// 			}
// 			if err := gateway.RegisterProxy(proxy); err != nil {
// 				log.Println("Failed registering proxy; error:", err)
// 			}
// 		}
// 	}()

// 	log.Println("Starting Infrared")
// 	if err := gateway.ListenAndServe(proxies); err != nil {
// 		log.Fatal("Gateway exited; error:", err)
// 	}

// 	gateway.KeepProcessActive()
// }
