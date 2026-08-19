// Command pawl is the CLI entry point.
//
// `make build` produces one static binary with no runtime dependency — the
// property the whole distribution story rests on (decision 6 in AGENTS.md).
package main

import (
	"os"

	"trunion.io/pawl/internal/cli"
)

// version is injected at build time by the Makefile:
//
//	-ldflags "-X main.version=$(VERSION)"
//
// It defaults to "dev" so a plain `go build` still runs. Once the attestation
// records the tool that produced it — see the known gaps in AGENTS.md — this is
// the value that goes in.
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
