package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func defaultConnect(ctx context.Context, dsn string) (dbClient, error) {
	return core.Connect(ctx, dsn)
}

func defaultRun(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func defaultNewPoller(client dbClient, interval time.Duration) *core.Poller {
	return core.NewPoller(client, interval)
}

var (
	connectFn     func(ctx context.Context, dsn string) (dbClient, error) = defaultConnect
	runFn         func(tea.Model) error                                    = defaultRun
	newPollerFn   func(dbClient, time.Duration) *core.Poller               = defaultNewPoller
	resolvePathFn = config.ResolvePath
	getWdFn       = os.Getwd
	initPathFn    = func() (string, error) {
		cwd, err := getWdFn()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".pgincident.toml"), nil
	}
)

const initContent = "dsn = \"postgres://user:password@localhost:5432/mydb\"\n\n[thresholds]\nlong_running        = \"5s\"\nidle_in_transaction = \"30s\"\n"

func Init(stdout io.Writer) error {
	path, err := initPathFn()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists", path)
		}
		return err
	}
	defer f.Close()
	fmt.Fprint(f, initContent)
	fmt.Fprintf(stdout, "Created %s\n", path)
	return nil
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
	if err == cli.ErrInit {
		if err := Init(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
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
	resolved, err := resolvePathFn(cfgPath)
	if err != nil {
		return err
	}

	cfg, err := config.Load(resolved)
	if err != nil {
		return err
	}

	if cfg.DSN == "" {
		return fmt.Errorf("no DSN configured — set 'dsn' in %s", resolved)
	}

	ctx := context.Background()

	client, err := connectFn(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	poller := newPollerFn(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	return runFn(tui.New(poller))
}
