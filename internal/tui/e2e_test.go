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

// TestGoldenDetailOverlay verifies the detail overlay layout against a stored golden file.
// Run with -update to regenerate: go test ./internal/tui/ -run TestGoldenDetailOverlay -update
func TestGoldenDetailOverlay(t *testing.T) {
	act := core.Activity{
		PID:      1001,
		User:     "alice",
		Duration: 12 * time.Second,
		State:    "active",
		Query:    "SELECT u.id, u.name, u.email, o.id AS order_id, o.status, o.total_amount, p.name AS product_name FROM users u JOIN orders o ON o.user_id = u.id JOIN order_items oi ON oi.order_id = o.id JOIN products p ON p.id = oi.product_id WHERE u.status = 'active' AND o.created_at > NOW() - INTERVAL '7 days' ORDER BY o.created_at DESC LIMIT 100",
	}
	app := &App{
		pollCh:     make(chan core.PollResult),
		cancel:     func() {},
		poller:     core.NewPoller(nil, time.Second),
		width:      80,
		height:     24,
		snapshot:   e2eSnapshot(),
		showDetail: true,
		detailItem: &act,
	}
	golden.RequireEqual(t, []byte(ansi.Strip(app.View())))
}

// TestE2EDetailOverlayOpenClose verifies that pressing Enter on an Activity row opens the
// detail overlay showing the full SQL, and pressing any key closes it.
func TestE2EDetailOverlayOpenClose(t *testing.T) {
	snap := e2eSnapshot()
	app := newE2EApp(t, snap)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Wait for initial render with the activity row visible.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("SELECT count(*) FROM orders"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Press Enter to open the detail overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// The detail overlay must show the full SQL and the PID.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("Query Detail")) &&
			bytes.Contains(s, []byte("1001")) &&
			bytes.Contains(s, []byte("[any key] close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Press any key to dismiss the overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// The main dashboard must be visible again (detail overlay gone).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("Long-running queries")) &&
			!bytes.Contains(s, []byte("Query Detail"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// TestE2EDetailOverlayLocksNoOp verifies that pressing Enter on the Locks section
// does not open the detail overlay. Inspects the final model state after quitting.
func TestE2EDetailOverlayLocksNoOp(t *testing.T) {
	snap := e2eSnapshot()
	snap.Locks = []core.Lock{{BlockedPID: 100, BlockingPID: 200, Relation: "public.orders"}}
	app := newE2EApp(t, snap)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	// Wait for initial render then navigate to Locks section.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("▶ Long-running queries"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("▶ Locks (waiting)"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Press Enter on Locks row (no-op), then quit to capture the final model.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	fa, ok := fm.(*App)
	if !ok {
		t.Fatal("expected *App as final model")
	}
	if fa.showDetail {
		t.Error("expected showDetail=false after Enter on Locks row")
	}
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
