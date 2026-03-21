package main

import (
	"fmt"
	"os"

	"github.com/traweezy/stackctl/cmd"
)

func main() {
	app := cmd.NewApp()
	if err := cmd.NewRootCmd(app).Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
