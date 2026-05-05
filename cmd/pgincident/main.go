package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"
	}

	ctx := context.Background()

	client, err := core.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, time.Second)
	app := tui.New(poller)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
