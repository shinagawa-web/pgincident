package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/config"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
)

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

	client, err := core.Connect(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	p := tea.NewProgram(tui.New(poller), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
