package main

import (
	"embed"
	"os"

	"github.com/haveachin/infrared/cmd"
)

//go:embed configs LICENSE LICENSE_NOTICES
var files embed.FS

func main() {
	if err := cmd.Execute(files); err != nil {
		os.Exit(1)
	}
}
