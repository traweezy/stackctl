package main

import (
	"fmt"
	"os"

	"github.com/traweezy/stackctl/cmd"
)

var (
	version   = "0.8.1"
	gitCommit = ""
	buildDate = ""
)

func main() {
	app := cmd.NewApp()
	app.Version = version
	app.GitCommit = gitCommit
	app.BuildDate = buildDate

	if err := cmd.NewRootCmd(app).Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
