package app

import (
	"context"
	"fmt"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/cli"
	"github.com/shinagawa-web/pgincident/internal/config"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
)

type dbClient interface {
	core.Querier
	Close(ctx context.Context)
}

var connectFn func(ctx context.Context, dsn string) (dbClient, error) = func(ctx context.Context, dsn string) (dbClient, error) {
	return core.Connect(ctx, dsn)
}

var runFn = func(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func Main(args []string, versionStr string, stdout, stderr io.Writer) int {
	cfgPath, err := cli.ParseFlags(args)
	if err == cli.ErrVersion {
		fmt.Fprintf(stdout, "pgincident v%s\n", versionStr)
		return 0
	}
	if err == cli.ErrHelp {
		fmt.Fprint(stdout, cli.HelpText)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := Run(cfgPath); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func Run(cfgPath string) error {
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if cfg.DSN == "" {
		return fmt.Errorf("no DSN configured — set 'dsn' in %s", cfgPath)
	}

	ctx := context.Background()

	client, err := connectFn(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	return runFn(tui.New(poller))
}
