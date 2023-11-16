package main

import (
	"log"

	ir "github.com/haveachin/infrared/pkg/infrared"
)

func main() {
	srv := ir.New(
		ir.WithBindAddr(":25565"),
		ir.AddServerConfig(
			ir.WithServerDomains("*"),
			ir.WithServerAddress(":25566"),
		),
	)

	log.Println(srv.ListenAndServe())
}
