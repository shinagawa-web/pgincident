package tui

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/shinagawa-web/pgincident/internal/core"
)

// newReconnectApp creates an App wired for reconnect tests.
// The returned channel is the write end of pollCh; send PollResult values to drive the app.
func newReconnectApp(
	t *testing.T,
	connName string,
	connList []ConnectionPreset,
	reconnectFn ReconnectFn,
) (*App, chan<- core.PollResult) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan core.PollResult, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
	}()
	app := &App{
		pollCh:               ch,
		cancel:               cancel,
		poller:               core.NewPoller(nil, time.Second),
		currentConn:          connName,
		connList:             connList,
		reconnectFn:          reconnectFn,
		pollerDone:           done,
		width:                120,
		height:               40,
		screen:               ScreenDashboard,
		reconnectMaxDuration: 10 * time.Minute,
	}
	t.Cleanup(cancel)
	return app, ch
}

// waitInitial waits for the first full render (title bar with "interval:" is always present).
func waitInitial(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("interval:"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 1: Watching dashboard — DB goes down → reconnecting ---

func TestCase1_DBFailureDashboardShowsReconnecting(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			return nil, errors.New("connection refused")
		},
	)
	// Prime with a snapshot so the dashboard has data before the failure.
	ch <- core.PollResult{Snapshot: core.Snapshot{
		PGVersion: "16.1", ServerAddr: "localhost:5432",
		Activities: []core.Activity{{PID: 1001, Query: "SELECT 1"}},
	}}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	// Trigger connection failure.
	ch <- core.PollResult{Err: errors.New("conn closed")}

	// Status must show "reconnecting…"; raw pgx error must NOT appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("reconnecting")) &&
			!bytes.Contains(s, []byte("conn closed"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Tool must remain interactive.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("press any key to close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 2: Watching overview — DB goes down → reconnecting ---

func TestCase2_DBFailureOverviewShowsReconnecting(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			return nil, errors.New("connection refused")
		},
	)
	app.screen = ScreenOverview

	ch <- core.PollResult{Snapshot: core.Snapshot{PGVersion: "16.1", ServerAddr: "localhost:5432"}}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("DB Health Overview"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	ch <- core.PollResult{Err: errors.New("conn closed")}

	// Overview must surface "reconnecting…"; raw error must not appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("reconnecting")) &&
			!bytes.Contains(s, []byte("conn closed"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Tool must remain interactive.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("press any key to close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 3: Reconnect succeeds → snapshot resumes, "connected: <name>" ---

func TestCase3_ReconnectSucceeds(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			return core.NewPoller(nil, time.Second), nil
		},
	)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("conn closed")}

	// After successful reconnect the status must show "connected: primary".
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("connected: primary"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 4: Reconnect keeps failing — reconnecting stays, app is interactive ---

func TestCase4_ReconnectFailsStatusStaysReconnecting(t *testing.T) {
	attempted := make(chan struct{}, 1)
	var once sync.Once

	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			once.Do(func() { close(attempted) })
			return nil, errors.New("connection refused")
		},
	)
	app.reconnectMaxDuration = 10 * time.Minute

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("conn closed")}

	// Wait for at least one reconnect attempt to complete (and fail).
	select {
	case <-attempted:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnectFn was not called within 5s after connection error")
	}

	// After the failed attempt, status must still show "reconnecting…".
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("reconnecting"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// App must be interactive.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("press any key to close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 5: 10-minute give-up → "connection lost", tool stays interactive ---

func TestCase5_ReconnectGivesUpAfterTimeout(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			return nil, errors.New("connection refused")
		},
	)
	app.reconnectMaxDuration = 100 * time.Millisecond // simulate 10-minute give-up

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("conn closed")}

	// After the deadline, status must show "connection lost".
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("connection lost"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Tool must remain interactive after giving up.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("press any key to close"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 6: Manual switch during reconnecting to a reachable connection ---

func TestCase6_ManualSwitchToReachableWhileReconnecting(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{
			{Name: "primary", DSN: "postgres://p"},
			{Name: "replica", DSN: "postgres://r"},
		},
		func(_ context.Context, dsn string) (*core.Poller, error) {
			if dsn == "postgres://p" {
				return nil, errors.New("connection refused")
			}
			// replica succeeds
			return core.NewPoller(nil, time.Second), nil
		},
	)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("conn closed")}

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("reconnecting"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// User opens connection selector and switches to replica.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("Select Connection"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// After switching to replica, "connected: replica" must appear and "reconnecting…" must stop.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("connected: replica"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 7: Manual switch during reconnecting to another unreachable connection ---

func TestCase7_ManualSwitchToUnreachableWhileReconnecting(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{
			{Name: "primary", DSN: "postgres://p"},
			{Name: "replica", DSN: "postgres://r"},
		},
		func(_ context.Context, _ string) (*core.Poller, error) {
			return nil, errors.New("connection refused")
		},
	)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("conn closed")}

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("reconnecting"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// User switches to replica (also unreachable).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("Select Connection"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// After switching, title must show "replica" and "reconnecting…" must reappear for replica.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		// "connected: replica" must NOT appear (replica is also unreachable).
		// "reconnecting" must appear, and "replica" must be visible in the title.
		return bytes.Contains(s, []byte("reconnecting")) &&
			bytes.Contains(s, []byte("replica"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 8: Non-connection error on dashboard → actual error shown, no reconnecting ---

func TestCase8_NonConnectionErrorDashboardShowsError(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		nil, // must NOT be called for non-connection errors
	)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	ch <- core.PollResult{Err: errors.New("permission denied for table pg_stat_activity")}

	// Actual error text must appear; "reconnecting…" must NOT appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("permission denied")) &&
			!bytes.Contains(s, []byte("reconnecting"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- Case 9: Non-connection error on overview → actual error shown, no reconnecting ---

func TestCase9_NonConnectionErrorOverviewShowsError(t *testing.T) {
	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		nil,
	)
	app.screen = ScreenOverview

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(stripped(bts), []byte("DB Health Overview"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	ch <- core.PollResult{Err: errors.New("permission denied for table pg_stat_activity")}

	// Actual error text must appear in the overview; "reconnecting…" must NOT appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripped(bts)
		return bytes.Contains(s, []byte("permission denied")) &&
			!bytes.Contains(s, []byte("reconnecting"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// --- U1: Double-reconnect guard ---

func TestU1_NoDoubleReconnect(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 1) // signals when reconnectFn is first entered
	blocking := make(chan struct{})   // closed by cleanup to unblock goroutines

	app, ch := newReconnectApp(t, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: "postgres://p"}},
		func(_ context.Context, _ string) (*core.Poller, error) {
			calls.Add(1)
			select {
			case started <- struct{}{}:
			default:
			}
			<-blocking
			return nil, errors.New("connection refused")
		},
	)
	t.Cleanup(func() { close(blocking) })

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })
	waitInitial(t, tm)

	// First error triggers reconnect (which will block on `blocking`).
	ch <- core.PollResult{Err: errors.New("conn closed")}

	// Wait for the reconnectFn to be entered.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnectFn was not called after first error")
	}

	// Send second error while first reconnect is still in progress.
	ch <- core.PollResult{Err: errors.New("conn closed again")}

	// If the guard works, reconnectFn should NOT be called a second time.
	select {
	case <-started:
		t.Errorf("reconnectFn called a second time (double-reconnect guard failed); total calls = %d", calls.Load())
	case <-time.After(200 * time.Millisecond):
		// Good: no second call within the window.
	}
}
