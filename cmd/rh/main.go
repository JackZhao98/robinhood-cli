package main

import (
	"github.com/jackzhao/robinhood-cli/internal/commands"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// Build metadata. All three are set at build time via the Makefile:
//
//   -ldflags "-X main.version=v1.2.1
//             -X main.commit=abc1234
//             -X main.builtAt=2026-04-30T19:00:00Z"
//
// Defaults are "dev" / "unknown" so a `go run` / `go install` without
// the Makefile still produces a self-describing binary instead of a
// silently empty version.
var (
	version = "dev"
	commit  = "unknown"
	builtAt = "unknown"
)

func main() {
	root := commands.NewRoot()
	root.Version = version
	commands.SetBuildInfo(version, commit, builtAt)
	if err := root.Execute(); err != nil {
		output.Fail(err)
	}
}
