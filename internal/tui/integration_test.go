package tui

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	return dsn
}

// TestIntegrationAutoReconnect verifies that the app detects a connection error,
// calls reconnectFn against the real DB, and resumes polling on the new poller.
func TestIntegrationAutoReconnect(t *testing.T) {
	dsn := integrationDSN(t)

	client, err := core.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	poller := core.NewPoller(client, time.Second)

	reconnectCalled := false
	reconnectFn := func(ctx context.Context, d string) (*core.Poller, error) {
		reconnectCalled = true
		newClient, err := core.Connect(ctx, d)
		if err != nil {
			return nil, err
		}
		t.Cleanup(func() { newClient.Close(context.Background()) })
		return core.NewPoller(newClient, time.Second), nil
	}
	fallbackFn := func(interval, lr, idle time.Duration) *core.Poller {
		return core.NewPoller(client, interval)
	}

	app := New(poller, "primary",
		[]ConnectionPreset{{Name: "primary", DSN: dsn}},
		reconnectFn, fallbackFn,
	)
	t.Cleanup(func() {
		app.cancel()
		<-app.pollerDone
	})

	// Simulate a connection error on the current generation.
	model, cmd := app.Update(snapshotMsg{
		PollResult: core.PollResult{Err: fmt.Errorf("conn closed")},
		gen:        app.gen,
	})
	a := model.(*App)
	if !a.autoReconnecting {
		t.Fatal("expected autoReconnecting=true after conn error")
	}
	if cmd == nil {
		t.Fatal("expected reconnect cmd")
	}

	// Execute the cmd: waits for old poller to stop, then calls real reconnectFn.
	result := cmd()
	reconnectMsg, ok := result.(autoReconnectResultMsg)
	if !ok {
		t.Fatalf("expected autoReconnectResultMsg, got %T", result)
	}
	if reconnectMsg.err != nil {
		t.Fatalf("expected successful reconnect against real DB, got: %v", reconnectMsg.err)
	}
	if !reconnectCalled {
		t.Error("reconnectFn was not called")
	}

	// Apply the successful result: new poller should start, autoReconnecting cleared.
	model2, cmd2 := a.Update(reconnectMsg)
	a2 := model2.(*App)
	t.Cleanup(func() {
		a2.cancel()
		<-a2.pollerDone
	})
	if a2.autoReconnecting {
		t.Error("expected autoReconnecting=false after successful reconnect")
	}
	if cmd2 == nil {
		t.Error("expected waitForSnapshot cmd after reconnect")
	}
	if a2.statusMsg != fmt.Sprintf("reconnected: %s", a2.currentConn) {
		t.Errorf("statusMsg = %q, want 'reconnected: primary'", a2.statusMsg)
	}
}

// TestIntegrationFallbackOnBadSwitch verifies that switching to an unreachable DB
// fails cleanly and the app resumes polling on the original connection via fallbackFn.
func TestIntegrationFallbackOnBadSwitch(t *testing.T) {
	dsn := integrationDSN(t)

	client, err := core.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { client.Close(context.Background()) })

	poller := core.NewPoller(client, time.Second)

	reconnectFn := func(ctx context.Context, d string) (*core.Poller, error) {
		newClient, err := core.Connect(ctx, d)
		if err != nil {
			return nil, err
		}
		t.Cleanup(func() { newClient.Close(context.Background()) })
		return core.NewPoller(newClient, time.Second), nil
	}
	fallbackFn := func(interval, lr, idle time.Duration) *core.Poller {
		return core.NewPoller(client, interval)
	}

	goodPreset := ConnectionPreset{Name: "primary", DSN: dsn}
	badPreset := ConnectionPreset{Name: "bad", DSN: "postgres://bad@localhost:19999/nope"}

	app := New(poller, "primary", []ConnectionPreset{goodPreset, badPreset}, reconnectFn, fallbackFn)
	t.Cleanup(func() {
		app.cancel()
		<-app.pollerDone
	})

	// Attempt to switch to the bad (unreachable) connection.
	cmd := app.doReconnect(badPreset)
	result := cmd()

	errMsg, ok := result.(reconnectErrMsg)
	if !ok {
		t.Fatalf("expected reconnectErrMsg for bad DSN, got %T", result)
	}
	if errMsg.err == nil {
		t.Fatal("expected a connection error for unreachable DSN")
	}
	if errMsg.fallback == nil {
		t.Fatal("expected a non-nil fallback poller")
	}

	// Apply the error: app should resume polling on the fallback.
	model, cmd2 := app.Update(errMsg)
	a := model.(*App)
	t.Cleanup(func() {
		a.cancel()
		<-a.pollerDone
	})
	if cmd2 == nil {
		t.Error("expected waitForSnapshot cmd from fallback")
	}
	if a.poller != errMsg.fallback {
		t.Error("expected fallback poller to be active after failed switch")
	}
	if a.lastErr == nil {
		t.Error("expected lastErr to be set after failed switch")
	}
}
