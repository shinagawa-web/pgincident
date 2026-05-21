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
	runFn         func(tea.Model) error                                   = defaultRun
	newPollerFn   func(dbClient, time.Duration) *core.Poller              = defaultNewPoller
	resolvePathFn                                                         = config.ResolvePath
	getWdFn                                                               = os.Getwd
	initPathFn                                                            = func() (string, error) {
		cwd, err := getWdFn()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".pgincident.toml"), nil
	}
	openInitFileFn = func(path string) (io.WriteCloser, error) {
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
)

func Init(stdout io.Writer) error {
	path, err := initPathFn()
	if err != nil {
		return err
	}
	f, err := openInitFileFn(path)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists", path)
		}
		return err
	}
	if _, err := fmt.Fprint(f, config.DefaultTOML()); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
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

	firstConn := cfg.Connections[cfg.ConnectionOrder[0]]
	ctx := context.Background()

	client, err := connectFn(ctx, firstConn.DSN)
	if err != nil {
		return err
	}

	poller := newPollerFn(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	// Build connection preset list for the TUI switcher.
	presets := make([]tui.ConnectionPreset, len(cfg.ConnectionOrder))
	for i, name := range cfg.ConnectionOrder {
		presets[i] = tui.ConnectionPreset{Name: name, DSN: cfg.Connections[name].DSN}
	}

	defer func() { client.Close(ctx) }()

	return runFn(tui.New(poller, cfg.ConnectionOrder[0], presets, buildReconnect(&client, poller), buildFallback(&client)))
}

func buildReconnect(clientPtr *dbClient, pol *core.Poller) tui.ReconnectFn {
	return func(rctx context.Context, dsn string) (*core.Poller, error) {
		newClient, err := connectFn(rctx, dsn)
		if err != nil {
			return nil, err
		}
		(*clientPtr).Close(rctx)
		*clientPtr = newClient
		p := newPollerFn(*clientPtr, pol.Interval())
		p.LongRunningThreshold = pol.LongRunningThreshold
		p.IdleInTxThreshold = pol.IdleInTxThreshold
		return p, nil
	}
}

// buildFallback returns a FallbackFn that creates a new Poller for the current
// DB client without reconnecting. Called when a connection switch fails and we
// need to resume polling the still-alive original connection.
func buildFallback(clientPtr *dbClient) tui.FallbackFn {
	return func(interval time.Duration, lr, idle time.Duration) *core.Poller {
		p := newPollerFn(*clientPtr, interval)
		p.LongRunningThreshold = lr
		p.IdleInTxThreshold = idle
		return p
	}
}
