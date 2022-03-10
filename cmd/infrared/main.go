package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/haveachin/infrared/api"

	"github.com/haveachin/infrared"
)

const (
	envPrefix               = "INFRARED_"
	envConfigPath           = envPrefix + "CONFIG_PATH"
	envReceiveProxyProtocol = envPrefix + "RECEIVE_PROXY_PROTOCOL"
	envApiEnabled           = envPrefix + "API_ENABLED"
	envApiBind              = envPrefix + "API_BIND"
	envPrometheusEnabled    = envPrefix + "PROMETHEUS_ENABLED"
	envPrometheusBind       = envPrefix + "PROMETHEUS_BIND"
)

const (
	clfConfigPath           = "config-path"
	clfReceiveProxyProtocol = "receive-proxy-protocol"
	clfPrometheusEnabled    = "enable-prometheus"
	clfPrometheusBind       = "prometheus-bind"
)

var (
	configPath           = "./configs"
	receiveProxyProtocol = false
	prometheusEnabled    = false
	prometheusBind       = ":9100"
	apiEnabled           = false
	apiBind              = "127.0.0.1:8080"
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
	receiveProxyProtocol = envBool(envReceiveProxyProtocol, receiveProxyProtocol)
	apiEnabled = envBool(envApiEnabled, apiEnabled)
	apiBind = envString(envApiBind, apiBind)
	prometheusEnabled = envBool(envPrometheusEnabled, prometheusEnabled)
	prometheusBind = envString(envPrometheusBind, prometheusBind)
}

func initFlags() {
	flag.StringVar(&configPath, clfConfigPath, configPath, "path of all proxy configs")
	flag.BoolVar(&receiveProxyProtocol, clfReceiveProxyProtocol, receiveProxyProtocol, "should accept proxy protocol")
	flag.BoolVar(&prometheusEnabled, clfPrometheusEnabled, prometheusEnabled, "should run prometheus client exposing metrics")
	flag.StringVar(&prometheusBind, clfPrometheusBind, prometheusBind, "bind address and/or port for prometheus")
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

	gateway := infrared.Gateway{ReceiveProxyProtocol: receiveProxyProtocol}
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

	if apiEnabled {
		go api.ListenAndServe(configPath, apiBind)
	}

	if prometheusEnabled {
		gateway.EnablePrometheus(prometheusBind)
	}

	log.Println("Starting Infrared")
	if err := gateway.ListenAndServe(proxies); err != nil {
		log.Fatal("Gateway exited; error: ", err)
	}

	gateway.KeepProcessActive()
}
