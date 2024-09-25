package main

import (
	"github.com/uyuni-project/minima/cmd"
)

// This variable should be set by the GoReleaser workflow using git tags.
// For local builds, you can set it using ldflags:
// go build -ldflags "-X main.version=<version number>" -o ./bin/ -v ./...
var version = "dev"

func main() {
	cmd.Execute(version)
}
