---
title: Architecture
weight: 10
---

# Architecture

Three-layer separation:

```
pgincident/
├── cmd/
│   └── pgincident/
│       └── main.go          # CLI entry — wires core + tui, calls os.Exit
├── internal/
│   ├── core/                # DB → Go structs (no formatting, no UI)
│   │   ├── client.go        # pgx connection wrapper
│   │   ├── activity.go      # pg_stat_activity → []Activity
│   │   ├── locks.go         # pg_locks → []Lock
│   │   ├── stats.go         # pg_stat_database → DBStats
│   │   ├── poller.go        # background polling loop
│   │   └── types.go         # Activity, Lock, DBStats, Snapshot
│   ├── tui/                 # Bubble Tea Model/View/Update
│   │   ├── app.go           # root model
│   │   ├── overview.go      # Level 1 overview screen
│   │   ├── activity_view.go
│   │   ├── locks_view.go
│   │   ├── idle_view.go
│   │   ├── style.go         # Lipgloss styles
│   │   └── format.go        # duration / padding helpers
│   └── version/
│       └── version.go
└── .github/workflows/
```

## Boundary rules

- **`core` knows nothing about the TUI.** No lipgloss, no Bubble Tea types, no formatting. Returns plain Go structs and `time.Duration`.
- **`tui` knows nothing about the SQL.** Receives structs from `core` and maps them to views.
- **`main.go` is thin.** It only wires `os.Args`/`os.Stdout`/`os.Stderr` and calls `os.Exit`. All logic lives in `internal/`.

## Update loop

- Default interval: 5 seconds. Adjustable with `+` / `-` (minimum 1 s).
- Poller runs in a background goroutine and sends `PollResult` to the TUI via channel. The TUI never blocks on the DB.
- Uses `time.NewTimer` (not `time.After`) to avoid timer leaks.
- TPS is skipped when `XactTotal` goes backward (server restart / `pg_stat_reset`).

## DB load

All polled views (`pg_stat_activity`, `pg_locks`, `pg_stat_database`) read from shared memory with no disk I/O. Each query typically completes in < 1 ms; total overhead is a few ms/s (< 0.1% CPU). A single persistent connection is reused — no per-poll connection cost.
