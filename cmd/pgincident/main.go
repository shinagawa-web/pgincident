package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/cli"
	"github.com/shinagawa-web/pgincident/internal/config"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
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

	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.DSN == "" {
		fmt.Fprintf(os.Stderr, "error: no DSN configured — set 'dsn' in %s\n", cfgPath)
		os.Exit(1)
	}

	ctx := context.Background()

	client, err := core.Connect(ctx, cfg.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	app := tui.New(poller)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
