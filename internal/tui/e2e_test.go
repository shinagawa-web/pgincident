package tui

import (
	"bytes"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/shinagawa-web/pgincident/internal/core"
)

func TestMain(m *testing.M) {
	os.Setenv("NO_COLOR", "1")
	os.Exit(m.Run())
}

func e2eSnapshot() core.Snapshot {
	return core.Snapshot{
		PGVersion:  "16.1",
		ServerAddr: "localhost:5432",
		DBStats: core.DBStats{
			ConnectionsActive: 3,
			ConnectionsMax:    100,
			CacheHitRatio:     0.99,
		},
		Activities: []core.Activity{
			{PID: 1001, User: "alice", Duration: 12 * time.Second, State: "active", Query: "SELECT count(*) FROM orders"},
		},
		Locks:    []core.Lock{},
		IdleInTx: []core.Activity{},
	}
}

// newE2EApp creates an App with a snapshot pre-buffered in the channel for teatest.
func newE2EApp(snap core.Snapshot) *App {
	ch := make(chan core.PollResult, 1)
	ch <- core.PollResult{Snapshot: snap}
	return &App{
		pollCh: ch,
		cancel: func() {},
		poller: core.NewPoller(nil, time.Second),
	}
}

// TestGoldenMainView verifies the full dashboard layout against a stored golden file.
// Run with -update to regenerate: go test ./internal/tui/ -run TestGoldenMainView -update
func TestGoldenMainView(t *testing.T) {
	app := &App{
		pollCh:   make(chan core.PollResult),
		cancel:   func() {},
		poller:   core.NewPoller(nil, time.Second),
		width:    120,
		height:   40,
		snapshot: e2eSnapshot(),
	}
	out := ansi.Strip(app.View())
	golden.RequireEqual(t, []byte(out))
}

// TestGoldenHelpOverlay verifies the help overlay layout against a stored golden file.
func TestGoldenHelpOverlay(t *testing.T) {
	app := &App{
		pollCh:   make(chan core.PollResult),
		cancel:   func() {},
		poller:   core.NewPoller(nil, time.Second),
		width:    120,
		height:   40,
		snapshot: e2eSnapshot(),
		showHelp: true,
	}
	out := ansi.Strip(app.View())
	golden.RequireEqual(t, []byte(out))
}

// TestE2EQuit verifies that pressing q causes the program to exit cleanly.
func TestE2EQuit(t *testing.T) {
	app := newE2EApp(e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("pgincident"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	finalApp, ok := fm.(*App)
	if !ok {
		t.Fatal("expected *App as final model")
	}
	if !finalApp.quitting {
		t.Error("expected quitting=true after q")
	}
}

// TestE2EHelpViaKey verifies that pressing ? opens the help overlay via the real program loop.
func TestE2EHelpViaKey(t *testing.T) {
	app := newE2EApp(e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("pgincident"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("quit"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// TestE2ETabNavigation verifies that Tab cycles through sections via the real program loop.
func TestE2ETabNavigation(t *testing.T) {
	app := newE2EApp(e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Long-running queries"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Locks (waiting)"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}
