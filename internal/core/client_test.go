package core

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func pgMonitorRow(ok bool) *mockPgxRow {
	return &mockPgxRow{
		scanFn: func(dest ...any) error {
			*dest[0].(*bool) = ok
			return nil
		},
	}
}

func pgMonitorErrRow(err error) *mockPgxRow {
	return &mockPgxRow{
		scanFn: func(_ ...any) error { return err },
	}
}

func newMonitorConn(row pgx.Row) *mockConn {
	return &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row { return row },
	}
}

// --- newClient ---

func TestNewClientSuccess(t *testing.T) {
	conn := newMonitorConn(pgMonitorRow(true))
	c, err := newClient(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Error("expected non-nil client")
	}
}

func TestNewClientQueryRowError(t *testing.T) {
	conn := newMonitorConn(pgMonitorErrRow(errors.New("query error")))
	_, err := newClient(context.Background(), conn)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !conn.closed {
		t.Error("expected conn to be closed on error")
	}
}

func TestNewClientNotPgMonitor(t *testing.T) {
	conn := newMonitorConn(pgMonitorRow(false))
	_, err := newClient(context.Background(), conn)
	if err == nil {
		t.Error("expected error for non-pg_monitor user")
	}
	if !conn.closed {
		t.Error("expected conn to be closed")
	}
}

// --- connectFn default body ---

func TestConnectFnDefaultBody(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled → pgx.Connect returns immediately with a context error
	_, err := connectFn(ctx, "host=localhost dbname=testdb")
	if err == nil {
		t.Skip("unexpectedly connected to a real database")
	}
}

// --- Connect ---

func TestConnect(t *testing.T) {
	orig := connectFn
	defer func() { connectFn = orig }()

	conn := newMonitorConn(pgMonitorRow(true))
	connectFn = func(_ context.Context, _ string) (pgConn, error) { return conn, nil }

	c, err := Connect(context.Background(), "postgres://localhost/test")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Error("expected non-nil client")
	}
}

func TestConnectError(t *testing.T) {
	orig := connectFn
	defer func() { connectFn = orig }()

	connectFn = func(_ context.Context, _ string) (pgConn, error) {
		return nil, errors.New("connection refused")
	}

	_, err := Connect(context.Background(), "postgres://localhost/test")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// --- Close ---

func TestClientClose(t *testing.T) {
	conn := &mockConn{}
	c := &Client{conn: conn}
	c.Close(context.Background())
	if !conn.closed {
		t.Error("expected conn.Close to be called")
	}
}

// --- connUser ---

func TestConnUser(t *testing.T) {
	cfg := &pgx.ConnConfig{}
	cfg.User = "testuser"
	conn := &mockConn{config: cfg}
	if got := connUser(conn); got != "testuser" {
		t.Errorf("connUser = %q, want testuser", got)
	}
}
