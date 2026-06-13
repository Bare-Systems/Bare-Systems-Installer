package main

import (
	"os"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/cli"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

var (
	versionValue = "dev"
	commitValue  = "unknown"
	dateValue    = "unknown"
)

func main() {
	version.Version = versionValue
	version.Commit = commitValue
	version.Date = dateValue

	app := cli.New()
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
