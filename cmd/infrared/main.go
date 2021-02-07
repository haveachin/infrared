package main

import (
	"flag"
	"github.com/haveachin/infrared"
	"log"
	"os"
	"strconv"
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

	var proxies []*infrared.Proxy
	for _, cfg := range cfgs {
		proxies = append(proxies, &infrared.Proxy{
			Config: cfg,
		})
	}

	outCfgs := make(chan *infrared.ProxyConfig)
	go func() {
		if err := infrared.WatchProxyConfigFolder(configPath, outCfgs); err != nil {
			log.Println("Failed watching config folder; error:", err)
			log.Println("SYSTEM FAILURE: CONFIG WATCHER FAILED")
		}
	}()

	gateway := infrared.Gateway{}
	go func() {
		for {
			cfg, ok := <-outCfgs
			if !ok {
				return
			}

			proxy := &infrared.Proxy{Config: cfg}
			if err := gateway.RegisterProxy(proxy); err != nil {
				log.Println("Failed registering proxy; error:", err)
			}
		}
	}()

	log.Println("Starting Infrared")
	if err := gateway.ListenAndServe(proxies); err != nil {
		log.Fatal("Gateway exited; error:", err)
	}
}
