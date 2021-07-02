package proxy_test

import (
	"testing"

	"github.com/haveachin/infrared/config"
	"github.com/haveachin/infrared/proxy"
)

func validateServer(t *testing.T, cfg config.ServerConfig, manager proxy.ProxyLaneManager) {
	proxyLane, ok := manager.Proxies[cfg.ListenTo]
	if !ok {
		t.Helper()
		t.Errorf("There was no ProxyLane for address %v", cfg.ListenTo)
	}
	serverMap := proxyLane.TestMethod_ServerMap()
	if _, ok := serverMap[cfg.MainDomain]; !ok {
		t.Helper()
		t.Error("Proxylane didnt contain a server with domain")
	}
}
func TestAddServerConfig(t *testing.T) {
	t.Run("Add Single server", func(t *testing.T) {
		manager := proxy.NewProxyLaneManager()
		serverCfg := config.ServerConfig{
			MainDomain: "infrared",
			ListenTo:   ":25565",
		}
		manager.ListenerFactory, _ = newTestListener()
		manager.AddServer(serverCfg)
		validateServer(t, serverCfg, manager)
	})

	t.Run("Add Multiple servers", func(t *testing.T) {
		serverCfg := config.ServerConfig{
			MainDomain: "infrared",
			ListenTo:   ":25565",
		}
		server2Cfg := config.ServerConfig{
			MainDomain: "infrared1",
			ListenTo:   ":25566",
		}
		manager := proxy.NewProxyLaneManager()
		manager.ListenerFactory, _ = newTestListener()
		manager.AddServer(serverCfg, server2Cfg)
		validateServer(t, serverCfg, manager)
		validateServer(t, server2Cfg, manager)
	})
}
