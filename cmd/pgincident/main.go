package main

import (
	"fmt"
	"os"

	"github.com/shinagawa-web/pgincident/internal/app"
	"github.com/shinagawa-web/pgincident/internal/cli"
	"github.com/shinagawa-web/pgincident/internal/version"
)

func main() {
	cfgPath, err := cli.ParseFlags(os.Args[1:])
	if err == cli.ErrVersion {
		fmt.Printf("pgincident v%s\n", version.Version)
		os.Exit(0)
	}
	if err == cli.ErrHelp {
		fmt.Print(cli.HelpText)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
