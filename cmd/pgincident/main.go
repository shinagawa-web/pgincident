package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"
	}

	ctx := context.Background()

	client, err := core.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, time.Second)
	out := make(chan core.PollResult, 1)
	go poller.Run(ctx, out)

	r := <-out
	if r.Err != nil {
		fmt.Fprintf(os.Stderr, "poll error: %v\n", r.Err)
		os.Exit(1)
	}
	s := r.Snapshot

	fmt.Printf("PG %s @ %s\n", s.PGVersion, s.ServerAddr)
	fmt.Printf("connections: %d/%d  TPS: %.1f  cache: %.1f%%\n",
		s.DBStats.ConnectionsActive, s.DBStats.ConnectionsMax,
		s.DBStats.TPS, s.DBStats.CacheHitRatio*100)
	fmt.Printf("long-running: %d  locks: %d  idle-in-tx: %d\n",
		len(s.Activities), len(s.Locks), len(s.IdleInTx))

	if len(s.Activities) > 0 {
		fmt.Printf("\n%-8s %-16s %-16s %s\n", "PID", "USER", "DURATION", "QUERY")
		for _, a := range s.Activities {
			fmt.Printf("%-8d %-16s %-16s %s\n", a.PID, a.User, formatDuration(a.Duration), truncate(a.Query, 60))
		}
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := d.Seconds() - float64(int(d.Minutes()))*60
	return fmt.Sprintf("%02d:%02d:%05.2f", h, m, s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
