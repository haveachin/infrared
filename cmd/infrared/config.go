package main

import (
	"bytes"
	_ "embed"
	"errors"
	"log"
	"os"
	"time"

	"github.com/haveachin/infrared"
	"github.com/spf13/viper"
)

const configPath = "config.yml"

//go:embed config.yml
var defaultConfig []byte

func init() {
	viper.SetConfigFile(configPath)
	viper.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err := viper.MergeInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

type gatewayConfig struct {
	Binds                []string      `mapstructure:"binds"`
	ReceiveProxyProtocol bool          `mapstructure:"receive_proxy_protocol"`
	ReceiveRealIP        bool          `mapstructure:"receive_real_ip"`
	ClientTimeout        time.Duration `mapstructure:"client_timeout"`
	Servers              []string      `mapstructure:"servers"`
}

func newGateway(id string, cfg gatewayConfig) (infrared.Gateway, error) {
	return infrared.Gateway{
		ID:                   id,
		Binds:                cfg.Binds,
		ReceiveProxyProtocol: cfg.ReceiveProxyProtocol,
		ReceiveRealIP:        cfg.ReceiveRealIP,
		ClientTimeout:        cfg.ClientTimeout,
		ServerIDs:            cfg.Servers,
	}, nil
}

func loadGateways() ([]infrared.Gateway, error) {
	var defaultCfg map[string]interface{}
	if err := viper.UnmarshalKey("defaults.gateway", &defaultCfg); err != nil {
		return nil, err
	}

	var gateways []infrared.Gateway
	for id := range viper.GetStringMap("gateways") {
		vpr := viper.Sub("gateways." + id)
		if err := vpr.MergeConfigMap(defaultCfg); err != nil {
			return nil, err
		}
		var cfg gatewayConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		gateway, err := newGateway(id, cfg)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}
