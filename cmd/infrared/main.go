package main

import (
	"log"

	ir "github.com/haveachin/infrared/pkg/infrared"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	srv := ir.New(
		ir.AddListenerConfig(
			ir.WithListenerBind(":25565"),
		),
		ir.AddServerConfig(
			ir.WithServerDomains("*"),
			ir.WithServerAddresses(":25566"),
		),
	)

	return srv.ListenAndServe()
}
