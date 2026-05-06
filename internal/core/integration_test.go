package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	c, err := Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	return c
}

func TestIntegrationServerInfo(t *testing.T) {
	c := integrationClient(t)
	version, addr, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version == "" {
		t.Error("version is empty")
	}
	if addr == ":" {
		t.Errorf("addr looks empty: %q", addr)
	}
}

func TestIntegrationStats(t *testing.T) {
	c := integrationClient(t)
	s, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.ConnectionsMax == 0 {
		t.Error("ConnectionsMax is 0")
	}
	if s.ConnectionsActive == 0 {
		t.Error("ConnectionsActive is 0 — at least our own connection should appear")
	}
	if s.XactTotal < 0 {
		t.Errorf("XactTotal = %d, want >= 0", s.XactTotal)
	}
	if s.CacheHitRatio < 0 || s.CacheHitRatio > 1 {
		t.Errorf("CacheHitRatio = %f, want [0, 1]", s.CacheHitRatio)
	}
}

func TestIntegrationLongRunning(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	c := integrationClient(t)
	ctx := context.Background()

	sleepConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("sleep connect: %v", err)
	}
	targetPID := int(sleepConn.PgConn().PID())

	sleepCtx, sleepCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sleepConn.Exec(sleepCtx, "SELECT pg_sleep(30)") //nolint:errcheck
	}()
	t.Cleanup(func() {
		sleepCancel()
		<-done
		sleepConn.Close(ctx) //nolint:errcheck
	})

	// threshold=1min: query just started, must not appear yet
	activities, err := c.LongRunning(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range activities {
		if a.PID == targetPID {
			t.Errorf("PID %d appeared with threshold=1min — threshold not filtering correctly", targetPID)
		}
	}

	// threshold=0: must appear once the query is visible in pg_stat_activity
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		activities, err := c.LongRunning(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range activities {
			if a.PID == targetPID {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("PID %d never appeared in LongRunning(threshold=0) within 5s", targetPID)
}

func TestIntegrationIdleInTx(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	c := integrationClient(t)
	ctx := context.Background()

	idleConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("idle connect: %v", err)
	}
	defer idleConn.Close(ctx) //nolint:errcheck

	if _, err = idleConn.Exec(ctx, "BEGIN"); err != nil {
		t.Fatalf("BEGIN: %v", err)
	}
	defer idleConn.Exec(ctx, "ROLLBACK") //nolint:errcheck

	targetPID := int(idleConn.PgConn().PID())

	// threshold=1min: transaction just started, must not appear yet
	activities, err := c.IdleInTx(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range activities {
		if a.PID == targetPID {
			t.Errorf("PID %d appeared with threshold=1min — threshold not filtering correctly", targetPID)
		}
	}

	// threshold=0: must appear
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		activities, err := c.IdleInTx(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range activities {
			if a.PID == targetPID {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("PID %d never appeared in IdleInTx(threshold=0) within 5s", targetPID)
}

func TestIntegrationLocks(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	c := integrationClient(t)
	ctx := context.Background()

	// conn1: acquire advisory transaction lock
	conn1, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("conn1: %v", err)
	}
	defer conn1.Close(ctx) //nolint:errcheck

	if _, err = conn1.Exec(ctx, "BEGIN"); err != nil {
		t.Fatalf("conn1 BEGIN: %v", err)
	}
	defer conn1.Exec(ctx, "ROLLBACK") //nolint:errcheck

	if _, err = conn1.Exec(ctx, "SELECT pg_advisory_xact_lock(12345)"); err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	blockingPID := int(conn1.PgConn().PID())

	// conn2: try to acquire the same lock — will block until conn1 commits/rolls back
	conn2, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("conn2: %v", err)
	}
	blockedPID := int(conn2.PgConn().PID())

	lockCtx, lockCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn2.Exec(lockCtx, "SELECT pg_advisory_xact_lock(12345)") //nolint:errcheck
	}()
	t.Cleanup(func() {
		lockCancel()
		<-done
		conn2.Close(ctx) //nolint:errcheck
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		locks, err := c.Locks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range locks {
			if l.BlockedPID == blockedPID && l.BlockingPID == blockingPID {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("lock pair (blocked=%d blocking=%d) never appeared within 5s", blockedPID, blockingPID)
}
