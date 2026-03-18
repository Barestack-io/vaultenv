package main

import (
	"os"

	"github.com/Barestack-io/vaultenv/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
