package main

import (
	"os"

	"github.com/shinagawa-web/pgincident/internal/app"
	"github.com/shinagawa-web/pgincident/internal/version"
)

func main() {
	os.Exit(app.Main(os.Args[1:], version.Version, os.Stdout, os.Stderr))
}
