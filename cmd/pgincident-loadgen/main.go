package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultDSN    = "postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"
	accountsCount = 10_000
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dsn          := flag.String("dsn", getenv("DATABASE_URL", defaultDSN), "PostgreSQL DSN")
	noTPS        := flag.Bool("no-tps", false, "disable background TPS workers")
	noSlow       := flag.Bool("no-slow", false, "disable slow-query workers")
	noLocks      := flag.Bool("no-locks", false, "disable lock contention workers")
	noIdle       := flag.Bool("no-idle", false, "disable idle-in-transaction workers")
	tpsWorkers   := flag.Int("tps-workers", 8, "number of background TPS workers")
	slowInterval := flag.Duration("slow-interval", 6*time.Second, "delay between slow queries per worker")
	idleDuration := flag.Duration("idle-duration", 45*time.Second, "how long each idle-in-transaction session lingers")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl-C: cancel context, then wait for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("interrupt received, shutting down...")
		cancel()
	}()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("connect pool: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	var enabled []string

	if !*noTPS {
		enabled = append(enabled, fmt.Sprintf("tps(%d workers)", *tpsWorkers))
		for i := 0; i < *tpsWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runTPSWorker(ctx, pool)
			}()
		}
	}

	if !*noSlow {
		enabled = append(enabled, "slow")
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(workerIdx int) {
				defer wg.Done()
				runSlowWorker(ctx, *dsn, *slowInterval, workerIdx)
			}(i)
		}
	}

	if !*noLocks {
		enabled = append(enabled, "locks")
		wg.Add(1)
		go func() {
			defer wg.Done()
			runLockWorker(ctx, *dsn)
		}()
	}

	if !*noIdle {
		enabled = append(enabled, "idle")
		// Two idle workers so one is always past the 30s threshold.
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(initialDelay time.Duration) {
				defer wg.Done()
				runIdleWorker(ctx, *dsn, *idleDuration, initialDelay)
			}(time.Duration(i) * (*idleDuration / 2))
		}
	}

	if len(enabled) == 0 {
		log.Println("all subsystems disabled — nothing to do")
		return
	}
	log.Printf("loadgen started: %v — Ctrl-C to stop", enabled)

	<-ctx.Done()
	wg.Wait()
	log.Println("loadgen stopped cleanly")
}

// runTPSWorker loops short OLTP SELECT/UPDATE queries on loadgen_accounts.
func runTPSWorker(ctx context.Context, pool *pgxpool.Pool) {
	for ctx.Err() == nil {
		id := rand.Int64N(accountsCount) + 1
		var err error
		if rand.IntN(3) == 0 {
			_, err = pool.Exec(ctx,
				`UPDATE loadgen_accounts SET balance = balance + $1, touched_at = now() WHERE id = $2`,
				rand.Float64()*10-5, id,
			)
		} else {
			var bal float64
			var ts time.Time
			err = pool.QueryRow(ctx,
				`SELECT balance, touched_at FROM loadgen_accounts WHERE id = $1`, id,
			).Scan(&bal, &ts)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("tps worker: %v", err)
			sleep(ctx, 100*time.Millisecond)
		}
	}
}

// runSlowWorker periodically runs a multi-second analytical query (> 5 s threshold).
// workerIdx staggers start so the two workers don't fire at the same time.
func runSlowWorker(ctx context.Context, dsn string, interval time.Duration, workerIdx int) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Printf("slow worker connect: %v", err)
		return
	}
	defer conn.Close(context.Background()) //nolint:errcheck

	// Stagger start to avoid simultaneous slow queries from both workers.
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Duration(workerIdx) * interval / 2):
	}

	for ctx.Err() == nil {
		sleepSecs := 6 + rand.IntN(6) // 6–11 s, always above the 5 s threshold
		idA := rand.Int64N(accountsCount) + 1
		idB := rand.Int64N(accountsCount) + 1
		_, err := conn.Exec(ctx,
			`WITH paused AS (SELECT pg_sleep($1))
			 SELECT a.id, a.balance, b.id, b.balance
			 FROM paused,
			      loadgen_accounts a
			 JOIN loadgen_accounts b ON b.id != a.id
			 WHERE a.id = $2
			   AND b.id = $3`,
			sleepSecs, idA, idB,
		)
		if err != nil && ctx.Err() == nil {
			log.Printf("slow worker query: %v", err)
		}

		// Jitter between runs so long-running rows rotate visibly.
		jitter := time.Duration(rand.Int64N(int64(interval)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter):
		}
	}
}

// runLockWorker loops lock-contention cycles: holder acquires a row lock, waiter
// blocks, then holder releases so the pair appears and resolves on the dashboard.
func runLockWorker(ctx context.Context, dsn string) {
	for ctx.Err() == nil {
		if err := lockCycle(ctx, dsn); err != nil && ctx.Err() == nil {
			log.Printf("lock worker: %v", err)
			sleep(ctx, time.Second)
		}
	}
}

func lockCycle(ctx context.Context, dsn string) error {
	holder, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("holder connect: %w", err)
	}
	defer func() {
		holder.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
		holder.Close(context.Background())             //nolint:errcheck
	}()

	waiter, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("waiter connect: %w", err)
	}
	defer func() {
		waiter.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
		waiter.Close(context.Background())             //nolint:errcheck
	}()

	if _, err := holder.Exec(ctx, "BEGIN"); err != nil {
		return fmt.Errorf("holder BEGIN: %w", err)
	}
	if _, err := holder.Exec(ctx, "SELECT id FROM loadgen_lock_rows WHERE id = 1 FOR UPDATE"); err != nil {
		return fmt.Errorf("holder lock: %w", err)
	}

	// Waiter tries to acquire the same row — will block until holder releases.
	waiterErrCh := make(chan error, 1)
	go func() {
		if _, err := waiter.Exec(ctx, "BEGIN"); err != nil {
			waiterErrCh <- err
			return
		}
		// This Exec blocks until holder commits/rolls back.
		_, err := waiter.Exec(ctx, "SELECT id FROM loadgen_lock_rows WHERE id = 1 FOR UPDATE")
		if err == nil {
			waiter.Exec(context.Background(), "COMMIT") //nolint:errcheck
		}
		waiterErrCh <- err
	}()

	// Hold the lock for 8–18 s so the dashboard has time to display the pair.
	holdFor := time.Duration(8+rand.IntN(10)) * time.Second
	sleep(ctx, holdFor)

	// Release the lock so the waiter can proceed.
	// Must wait for the goroutine before returning: pgx.Conn is not goroutine-safe,
	// and the defer ROLLBACK would race with the goroutine's in-flight Exec.
	holder.Exec(context.Background(), "ROLLBACK") //nolint:errcheck

	// pgx propagates ctx cancellation into the blocked Exec, so this always resolves.
	if err := <-waiterErrCh; err != nil && ctx.Err() == nil {
		log.Printf("lock waiter: %v", err)
	}

	// Pause between cycles.
	sleep(ctx, time.Duration(3+rand.IntN(5))*time.Second)
	return nil
}

// runIdleWorker opens a transaction, runs a query, then sits idle-in-transaction
// for idleDuration before committing/rolling back and repeating.
// initialDelay staggers the two idle workers so one is always past the 30 s threshold.
func runIdleWorker(ctx context.Context, dsn string, idleDuration, initialDelay time.Duration) {
	sleep(ctx, initialDelay)
	for ctx.Err() == nil {
		if err := idleCycle(ctx, dsn, idleDuration); err != nil && ctx.Err() == nil {
			log.Printf("idle worker: %v", err)
			sleep(ctx, time.Second)
		}
	}
}

func idleCycle(ctx context.Context, dsn string, idleDuration time.Duration) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("idle connect: %w", err)
	}
	// Use Background for cleanup so ROLLBACK is always sent even after ctx cancel.
	defer conn.Close(context.Background()) //nolint:errcheck

	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		return err
	}
	id := rand.Int64N(accountsCount) + 1
	if _, err := conn.Exec(ctx, "SELECT balance FROM loadgen_accounts WHERE id = $1", id); err != nil {
		conn.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
		return err
	}

	// Sit idle in transaction — the TUI threshold is 30 s.
	select {
	case <-ctx.Done():
		conn.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
		return nil
	case <-time.After(idleDuration):
	}

	if rand.IntN(2) == 0 {
		conn.Exec(context.Background(), "COMMIT")   //nolint:errcheck
	} else {
		conn.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
	}

	sleep(ctx, 2*time.Second)
	return nil
}

// sleep waits for d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
