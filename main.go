package main

import (
	"embed"
	"os"

	"github.com/haveachin/infrared/cmd"
)

//go:embed configs LICENSE LICENSE_NOTICES
var files embed.FS
var version = "devbuild"

func main() {
	if err := cmd.Execute(files, version); err != nil {
		os.Exit(1)
	}
}
