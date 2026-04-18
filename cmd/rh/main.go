package main

import (
	"github.com/jackzhao/robinhood-cli/internal/commands"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func main() {
	if err := commands.NewRoot().Execute(); err != nil {
		output.Fail(err)
	}
}
