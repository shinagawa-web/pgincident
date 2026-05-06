package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func newLocksConn(rows pgx.Rows, queryErr error) *mockConn {
	return &mockConn{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, queryErr
		},
	}
}

func TestLocksEmpty(t *testing.T) {
	c := &Client{conn: newLocksConn(&mockPgxRows{}, nil)}
	locks, err := c.Locks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 0 {
		t.Errorf("got %d locks, want 0", len(locks))
	}
}

func TestLocksWithData(t *testing.T) {
	rows := &mockPgxRows{
		data: [][]any{
			{101, 202, 3 * time.Second, "public.orders", "RowShareLock", "relation"},
		},
	}
	c := &Client{conn: newLocksConn(rows, nil)}
	locks, err := c.Locks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 {
		t.Fatalf("got %d locks, want 1", len(locks))
	}
	l := locks[0]
	if l.BlockedPID != 101 || l.BlockingPID != 202 {
		t.Errorf("unexpected pids: blocked=%d blocking=%d", l.BlockedPID, l.BlockingPID)
	}
	if l.WaitTime != 3*time.Second {
		t.Errorf("WaitTime = %v, want 3s", l.WaitTime)
	}
	if l.Relation != "public.orders" {
		t.Errorf("Relation = %q, want public.orders", l.Relation)
	}
}

func TestLocksQueryError(t *testing.T) {
	c := &Client{conn: newLocksConn(nil, errors.New("db error"))}
	_, err := c.Locks(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestLocksScanError(t *testing.T) {
	rows := &mockPgxRows{
		data:    [][]any{{101, 202, 3 * time.Second, "t", "m", "l"}},
		scanErr: errors.New("scan error"),
	}
	c := &Client{conn: newLocksConn(rows, nil)}
	_, err := c.Locks(context.Background())
	if err == nil {
		t.Error("expected scan error, got nil")
	}
}

func TestLocksRowErr(t *testing.T) {
	rows := &mockPgxRows{err: errors.New("connection reset")}
	c := &Client{conn: newLocksConn(rows, nil)}
	_, err := c.Locks(context.Background())
	if err == nil {
		t.Error("expected rows.Err() to propagate")
	}
}
