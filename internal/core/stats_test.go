package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

func newQueryRowConn(row pgx.Row) *mockConn {
	return &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row { return row },
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	call := 0
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			call++
			if call == 1 {
				return &mockPgxRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 160001 // PG16
					return nil
				}}
			}
			return &mockPgxRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 10
					*dest[1].(*int) = 100
					*dest[2].(*int64) = 5000
					*dest[3].(*float64) = 0.99
					*dest[4].(*int64) = 3
					*dest[5].(*bool) = true
					*dest[6].(*float64) = 12.4
					*dest[7].(*int) = 2
					return nil
				},
			}
		},
	}
	c := &Client{conn: conn}
	s, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.ConnectionsActive != 10 {
		t.Errorf("ConnectionsActive = %d, want 10", s.ConnectionsActive)
	}
	if s.ConnectionsMax != 100 {
		t.Errorf("ConnectionsMax = %d, want 100", s.ConnectionsMax)
	}
	if s.XactTotal != 5000 {
		t.Errorf("XactTotal = %d, want 5000", s.XactTotal)
	}
	if s.CacheHitRatio != 0.99 {
		t.Errorf("CacheHitRatio = %v, want 0.99", s.CacheHitRatio)
	}
	if s.CheckpointReq != 3 {
		t.Errorf("CheckpointReq = %d, want 3", s.CheckpointReq)
	}
	if !s.HasStandbys {
		t.Error("HasStandbys = false, want true")
	}
	if s.ReplicationLagSecs != 12.4 {
		t.Errorf("ReplicationLagSecs = %v, want 12.4", s.ReplicationLagSecs)
	}
	if s.AutovacuumWorkers != 2 {
		t.Errorf("AutovacuumWorkers = %d, want 2", s.AutovacuumWorkers)
	}
}

func TestStatsPG17(t *testing.T) {
	call := 0
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			call++
			if call == 1 {
				return &mockPgxRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 170000 // PG17
					return nil
				}}
			}
			return &mockPgxRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 5
					*dest[1].(*int) = 200
					*dest[2].(*int64) = 1000
					*dest[3].(*float64) = 0.95
					*dest[4].(*int64) = 1
					*dest[5].(*bool) = false
					*dest[6].(*float64) = 0
					*dest[7].(*int) = 0
					return nil
				},
			}
		},
	}
	c := &Client{conn: conn}
	s, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.ConnectionsActive != 5 {
		t.Errorf("ConnectionsActive = %d, want 5", s.ConnectionsActive)
	}
}

func TestStatsVersionError(t *testing.T) {
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockPgxRow{scanFn: func(_ ...any) error { return errors.New("version error") }}
		},
	}
	c := &Client{conn: conn}
	_, err := c.Stats(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestStatsError(t *testing.T) {
	call := 0
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			call++
			if call == 1 {
				return &mockPgxRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 160001
					return nil
				}}
			}
			return &mockPgxRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	c := &Client{conn: conn}
	_, err := c.Stats(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// --- ServerInfo ---

func TestServerInfo(t *testing.T) {
	call := 0
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			call++
			return &mockPgxRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "16.1"
					return nil
				},
			}
		},
	}
	cfg := &pgx.ConnConfig{}
	cfg.Host = "db.example.com"
	cfg.Port = 5432
	conn.config = cfg

	c := &Client{conn: conn}
	version, addr, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != "16.1" {
		t.Errorf("version = %q, want 16.1", version)
	}
	want := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if addr != want {
		t.Errorf("addr = %q, want %q", addr, want)
	}
}

func TestServerInfoStripsOSSuffix(t *testing.T) {
	conn := &mockConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockPgxRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "17.10 (Debian 17.10-1.pgdg13+1)"
					return nil
				},
			}
		},
	}
	cfg := &pgx.ConnConfig{}
	cfg.Host = "localhost"
	cfg.Port = 5433
	conn.config = cfg

	c := &Client{conn: conn}
	version, _, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != "17.10" {
		t.Errorf("version = %q, want 17.10", version)
	}
}

func TestServerInfoError(t *testing.T) {
	row := &mockPgxRow{
		scanFn: func(_ ...any) error { return errors.New("query error") },
	}
	c := &Client{conn: newQueryRowConn(row)}
	_, _, err := c.ServerInfo(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}
