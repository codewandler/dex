package main

import "dex/internal/cli"

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	cli.SetVersion(Version)
	cli.Execute()
}
