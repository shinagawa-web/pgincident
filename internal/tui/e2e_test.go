package tui

import (
	"bytes"
	"context"
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
	prev, hadPrev := os.LookupEnv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if hadPrev {
			os.Setenv("NO_COLOR", prev)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()
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

// newE2EApp creates an App that continuously re-sends a fixed snapshot so that
// waitForSnapshot never blocks after the first poll cycle. The background
// goroutine stops when ctx is cancelled (i.e. when the test calls cancel or tm.Quit).
func newE2EApp(t *testing.T, snap core.Snapshot) *App {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan core.PollResult, 1)
	go func() {
		for {
			select {
			case ch <- core.PollResult{Snapshot: snap}:
			case <-ctx.Done():
				return
			}
		}
	}()
	app := &App{
		pollCh: ch,
		cancel: cancel,
		poller: core.NewPoller(nil, time.Second),
	}
	t.Cleanup(cancel)
	return app
}

func stripped(bts []byte) []byte {
	return []byte(ansi.Strip(string(bts)))
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
	golden.RequireEqual(t, []byte(ansi.Strip(app.View())))
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
	golden.RequireEqual(t, []byte(ansi.Strip(app.View())))
}

// TestE2EQuit verifies that pressing q causes the program to exit cleanly.
func TestE2EQuit(t *testing.T) {
	app := newE2EApp(t, e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("pgincident"))
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
// Asserts on "press any key to close" which only appears inside the help overlay modal.
func TestE2EHelpViaKey(t *testing.T) {
	app := newE2EApp(t, e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("pgincident"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	// "press any key to close" only appears in the help overlay, not in the main view.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("press any key to close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// TestE2ETabNavigation verifies that Tab moves the active-section marker to Locks.
// Asserts on "▶ Locks (waiting)" which only appears when that section is focused.
func TestE2ETabNavigation(t *testing.T) {
	app := newE2EApp(t, e2eSnapshot())
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Wait for initial render with Activity section active.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("▶ Long-running queries"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	// After Tab, the active marker must move to Locks.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("▶ Locks (waiting)"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}
