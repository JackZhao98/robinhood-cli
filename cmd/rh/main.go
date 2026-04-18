package main

import (
	"github.com/jackzhao/robinhood-cli/internal/commands"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// Set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := commands.NewRoot()
	root.Version = version
	if err := root.Execute(); err != nil {
		output.Fail(err)
	}
}
