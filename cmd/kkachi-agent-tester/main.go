package main

import (
	"os"

	"kkachi-agent-tester/internal/cli"
)

var (
	version   = "0.1.0"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.NewBuildInfo("kkachi-agent-tester", version, commit, buildDate)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
