package main

import (
	"os"

	"github.com/Barestack-io/vaultenv/internal/commands"
)

// Set at link time: -ldflags "-X main.version=... -X main.commit=... -X main.date=..."
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands.SetBuildInfo(version, commit, date)
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
