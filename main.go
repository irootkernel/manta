package main

import (
	"os"

	"github.com/irootkernel/manta/internal/cli"
)

var (
	version   = "0.1.5"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.NewBuildInfo("manta", version, commit, buildDate)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
