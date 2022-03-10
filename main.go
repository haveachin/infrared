package main

import (
	"os"

	"github.com/haveachin/infrared/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
