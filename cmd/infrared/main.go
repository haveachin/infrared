package main

import (
	"log"

	"github.com/haveachin/infrared"
)

func main() {
	proxy := infrared.Proxy{
		Addr:    ":25565",
		Configs: map[string]infrared.Config{},
	}

	configs, err := infrared.ReadConfigs("./configs/")
	if err != nil {
		log.Fatal(err)
		return
	}

	for _, config := range configs {
		proxy.Configs[config.DomainName] = config
	}

	log.Fatal(proxy.ListenAndServe())
}
