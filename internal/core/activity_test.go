package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- scanActivities ---

func TestScanActivitiesEmpty(t *testing.T) {
	rows := &mockPgxRows{}
	activities, err := scanActivities(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 {
		t.Errorf("got %d activities, want 0", len(activities))
	}
	if !rows.closed {
		t.Error("rows.Close() not called")
	}
}

func TestScanActivitiesMultipleRows(t *testing.T) {
	now := time.Now()
	rows := &mockPgxRows{
		data: [][]any{
			{1234, "alice", "mydb", "active", now, 5 * time.Second, "SELECT 1", "app", "127.0.0.1"},
			{5678, "bob", "mydb", "active", now, 10 * time.Second, "SELECT 2", "app", "127.0.0.1"},
		},
	}
	activities, err := scanActivities(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(activities))
	}
	if activities[0].PID != 1234 || activities[0].User != "alice" {
		t.Errorf("unexpected first row: %+v", activities[0])
	}
	if activities[1].Duration != 10*time.Second {
		t.Errorf("unexpected duration: %v", activities[1].Duration)
	}
}

func TestScanActivitiesScanError(t *testing.T) {
	now := time.Now()
	rows := &mockPgxRows{
		data: [][]any{
			{1234, "alice", "mydb", "active", now, 5 * time.Second, "SELECT 1", "app", "127.0.0.1"},
		},
		scanErr: errors.New("scan error"),
	}
	_, err := scanActivities(rows)
	if err == nil {
		t.Error("expected scan error, got nil")
	}
}

func TestScanActivitiesRowErr(t *testing.T) {
	rows := &mockPgxRows{err: errors.New("connection reset")}
	_, err := scanActivities(rows)
	if err == nil {
		t.Error("expected rows.Err() to be returned")
	}
}

// --- LongRunning ---

func newActivityConn(rows pgx.Rows, queryErr error) *mockConn {
	return &mockConn{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, queryErr
		},
	}
}

func TestLongRunningEmpty(t *testing.T) {
	c := &Client{conn: newActivityConn(&mockPgxRows{}, nil)}
	acts, err := c.LongRunning(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 0 {
		t.Errorf("got %d activities, want 0", len(acts))
	}
}

func TestLongRunningWithData(t *testing.T) {
	now := time.Now()
	rows := &mockPgxRows{
		data: [][]any{
			{42, "alice", "mydb", "active", now, 6 * time.Second, "SELECT 1", "app", "127.0.0.1"},
		},
	}
	c := &Client{conn: newActivityConn(rows, nil)}
	acts, err := c.LongRunning(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].PID != 42 {
		t.Errorf("unexpected result: %+v", acts)
	}
}

func TestLongRunningQueryError(t *testing.T) {
	c := &Client{conn: newActivityConn(nil, errors.New("db error"))}
	_, err := c.LongRunning(context.Background(), time.Second)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// --- IdleInTx ---

func TestIdleInTxEmpty(t *testing.T) {
	c := &Client{conn: newActivityConn(&mockPgxRows{}, nil)}
	acts, err := c.IdleInTx(context.Background(), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 0 {
		t.Errorf("got %d activities, want 0", len(acts))
	}
}

func TestIdleInTxWithData(t *testing.T) {
	now := time.Now()
	rows := &mockPgxRows{
		data: [][]any{
			{99, "bob", "testdb", "idle in transaction", now, 35 * time.Second, "BEGIN", "psql", "(local)"},
		},
	}
	c := &Client{conn: newActivityConn(rows, nil)}
	acts, err := c.IdleInTx(context.Background(), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].PID != 99 {
		t.Errorf("unexpected result: %+v", acts)
	}
}

func TestIdleInTxQueryError(t *testing.T) {
	c := &Client{conn: newActivityConn(nil, errors.New("db error"))}
	_, err := c.IdleInTx(context.Background(), 30*time.Second)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
