package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
	"github.com/shinagawa-web/pgincident/internal/version"
)

const defaultDSN = "postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"

const helpText = `Usage: pgincident [options]

Options:
  -v, --version   Print version and exit
  -h, --help      Show this help

Environment:
  DATABASE_URL    PostgreSQL DSN (e.g. postgres://user:password@localhost:5432/mydb)
`

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-v", "--version":
			fmt.Printf("pgincident v%s\n", version.Version)
			os.Exit(0)
		case "-h", "--help":
			fmt.Print(helpText)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n%s", arg, helpText)
			os.Exit(1)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx := context.Background()

	client, err := core.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, 5*time.Second)
	app := tui.New(poller)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
