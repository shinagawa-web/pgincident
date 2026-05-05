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

	fmt.Println("connected. fetching long-running queries (> 5s)...")

	activities, err := client.LongRunning(ctx, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}

	if len(activities) == 0 {
		fmt.Println("(none)")
		return
	}

	fmt.Printf("%-8s %-16s %-16s %-14s %s\n", "PID", "USER", "DURATION", "STATE", "QUERY")
	for _, a := range activities {
		fmt.Printf("%-8d %-16s %-16s %-14s %s\n",
			a.PID, a.User,
			formatDuration(a.Duration),
			a.State,
			truncate(a.Query, 60),
		)
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
