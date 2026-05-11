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
	dsn        := flag.String("dsn", getenv("DATABASE_URL", defaultDSN), "PostgreSQL DSN")
	noTPS      := flag.Bool("no-tps", false, "disable background TPS workers")
	noSlow     := flag.Bool("no-slow", false, "disable slow-query workers")
	noLocks    := flag.Bool("no-locks", false, "disable lock contention workers")
	noIdle     := flag.Bool("no-idle", false, "disable idle-in-transaction workers")
	tpsWorkers := flag.Int("tps-workers", 8, "number of background TPS workers")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("interrupt received, shutting down...")
		cancel()
	}()

	var wg sync.WaitGroup
	var enabled []string

	if !*noTPS {
		pool, err := pgxpool.New(ctx, *dsn)
		if err != nil {
			log.Fatalf("connect pool: %v", err)
		}
		defer pool.Close()

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
		// Short: 2 workers, 6–11 s queries cycling quickly, staggered by 5 s.
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(delay time.Duration) {
				defer wg.Done()
				runSlowWorker(ctx, *dsn, 6, 11, 10*time.Second, delay)
			}(time.Duration(i) * 5 * time.Second)
		}
		// Long: 2 workers staggered by ~3 min. Worker 0 starts immediately;
		// worker 1 starts after half the average query duration (390 s / 2 = 195 s)
		// so the two overlap for roughly half of each cycle.
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(delay time.Duration) {
				defer wg.Done()
				runSlowWorker(ctx, *dsn, 300, 480, 30*time.Second, delay)
			}(time.Duration(i) * 195 * time.Second)
		}
	}

	if !*noLocks {
		enabled = append(enabled, "locks")
		// Short: row 1, 8–18 s hold, cycles quickly.
		wg.Add(1)
		go func() {
			defer wg.Done()
			runLockWorker(ctx, *dsn, 1, 8*time.Second, 18*time.Second, 5*time.Second)
		}()
		// Long: row 2, 5–8 min hold, stays blocked for minutes.
		wg.Add(1)
		go func() {
			defer wg.Done()
			runLockWorker(ctx, *dsn, 2, 300*time.Second, 480*time.Second, 30*time.Second)
		}()
	}

	if !*noIdle {
		enabled = append(enabled, "idle")
		// Short: 2 workers staggered by 22.5 s so one is always past the 30 s threshold.
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(initialDelay time.Duration) {
				defer wg.Done()
				runIdleWorker(ctx, *dsn, 45*time.Second, initialDelay)
			}(time.Duration(i) * 22 * time.Second)
		}
		// Long: 2 workers staggered by ~3.5 min (half the average 7 min duration)
		// so at least one is always visible, with an overlap window where 2 appear.
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(delay time.Duration) {
				defer wg.Done()
				runIdleWorker(ctx, *dsn, time.Duration(360+rand.IntN(120))*time.Second, delay)
			}(time.Duration(i) * 210 * time.Second)
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

// runSlowWorker periodically runs analytical queries sleeping for [minSecs, maxSecs].
// gap is the pause between consecutive queries.
// initialDelay staggers workers so they don't all fire at the same time.
func runSlowWorker(ctx context.Context, dsn string, minSecs, maxSecs int, gap, initialDelay time.Duration) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Printf("slow worker connect: %v", err)
		return
	}
	defer conn.Close(context.Background()) //nolint:errcheck

	sleep(ctx, initialDelay)

	for ctx.Err() == nil {
		sleepSecs := minSecs + rand.IntN(maxSecs-minSecs+1)
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
			log.Printf("slow worker (range %d-%ds): %v", minSecs, maxSecs, err)
		}
		sleep(ctx, gap)
	}
}

// runLockWorker loops lock-contention cycles on the given row.
// holdMin/holdMax control how long the holder keeps the lock.
// pause is the gap between cycles.
func runLockWorker(ctx context.Context, dsn string, rowID int, holdMin, holdMax, pause time.Duration) {
	for ctx.Err() == nil {
		if err := lockCycle(ctx, dsn, rowID, holdMin, holdMax); err != nil && ctx.Err() == nil {
			log.Printf("lock worker (row %d): %v", rowID, err)
			sleep(ctx, time.Second)
		}
		sleep(ctx, pause)
	}
}

func lockCycle(ctx context.Context, dsn string, rowID int, holdMin, holdMax time.Duration) error {
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
	if _, err := holder.Exec(ctx, "SELECT id FROM loadgen_lock_rows WHERE id = $1 FOR UPDATE", rowID); err != nil {
		return fmt.Errorf("holder lock row %d: %w", rowID, err)
	}

	// Waiter tries to acquire the same row — blocks until holder releases.
	waiterErrCh := make(chan error, 1)
	go func() {
		if _, err := waiter.Exec(ctx, "BEGIN"); err != nil {
			waiterErrCh <- err
			return
		}
		_, err := waiter.Exec(ctx, "SELECT id FROM loadgen_lock_rows WHERE id = $1 FOR UPDATE", rowID)
		if err == nil {
			waiter.Exec(context.Background(), "COMMIT") //nolint:errcheck
		}
		waiterErrCh <- err
	}()

	holdFor := holdMin + time.Duration(rand.Int64N(int64(holdMax-holdMin)))
	sleep(ctx, holdFor)

	// Release. Must drain waiterErrCh before returning: pgx.Conn is not
	// goroutine-safe, and the defer ROLLBACK would race with the goroutine's Exec.
	holder.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
	if err := <-waiterErrCh; err != nil && ctx.Err() == nil {
		log.Printf("lock waiter (row %d): %v", rowID, err)
	}
	return nil
}

// runIdleWorker opens a transaction, runs a query, then sits idle-in-transaction
// for idleDuration before cycling. initialDelay staggers workers.
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
	defer conn.Close(context.Background()) //nolint:errcheck

	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		return err
	}
	id := rand.Int64N(accountsCount) + 1
	if _, err := conn.Exec(ctx, "SELECT balance FROM loadgen_accounts WHERE id = $1", id); err != nil {
		conn.Exec(context.Background(), "ROLLBACK") //nolint:errcheck
		return err
	}

	// Sit idle in transaction — TUI threshold is 30 s.
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
