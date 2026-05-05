# Postgres Incident TUI — v0.1 Design

> Codename: TBD (named last)
> Status: Draft for review

A modern Postgres monitoring TUI focused on incident response. Inspired by `pgcenter` and `pg_activity`, but built with Bubble Tea and designed for managed PostgreSQL (RDS, Cloud SQL, Aurora) without requiring SUPERUSER.

---

## 1. Product Definition

### 1.1 Positioning

> "The first 30 seconds of a Postgres incident — in one screen."

Targets SREs and Web engineers who reach for `psql -c "SELECT * FROM pg_stat_activity"` when production gets slow. Replaces a sequence of manual queries with a single live dashboard.

### 1.2 Differentiation from competitors

| | pgcenter | pg_activity | This project |
|---|---|---|---|
| Language | Go | Python | Go |
| TUI library | ncurses (gocui) | curses | **Bubble Tea** |
| OS | Linux only | Linux/Mac | **Linux + Mac** |
| Required privilege | SUPERUSER | SUPERUSER | **`pg_monitor`** |
| Managed DB (RDS, Cloud SQL) | partial | partial | **first-class** |
| UX style | classic terminal | classic terminal | **modern (mouse, search, filter)** |
| Focus | comprehensive stats | top-style activity | **incident response** |
| Web UI | no | no | **planned (v1.0+)** |

The two big bets:

1. **`pg_monitor` instead of SUPERUSER** — unlocks managed PostgreSQL.
2. **Incident-response framing** — not "show me everything", but "what's broken right now."

### 1.3 Non-goals (v0.1)

- Replacing pgAdmin / DBeaver (no schema browsing, no query editor)
- Long-term metrics storage (Prometheus, Grafana already do this)
- Alerting (later version)
- Replication monitoring (later version)
- System stats (CPU/IO/mem) — pgcenter does this; we focus on Postgres internals
- Multi-instance dashboard (one connection at a time in v0.1)

---

## 2. v0.1 Feature Scope

### 2.1 Single-screen Incident Dashboard

```
┌─ <name> v0.1.0 ───────────── connected: prod-db@10.0.1.42:5432 (PG 16.1) ─┐
│ Connections: 142 / 200 (71%)   TPS: 2,340   Cache hit: 99.2%            │
├──────────────────────────────────────────────────────────────────────────┤
│ 🔥 Long-running queries (> 5s)                              [12 active] │
│   pid    user        duration     state    query                        │
│ ▸ 12345  app_user    00:02:14.32  active   SELECT u.* FROM users u JOIN…│
│   12346  worker      00:00:18.04  active   UPDATE jobs SET status=...   │
│   12347  reporting   00:00:08.91  active   SELECT count(*) FROM events  │
├──────────────────────────────────────────────────────────────────────────┤
│ 🔒 Locks (waiting)                                          [3 waiting] │
│   blocked  blocking  wait_time    relation       mode                   │
│ ▸ 12350    12345     00:01:23.10  public.users   ShareLock              │
│   12351    12345     00:01:15.40  public.users   ShareLock              │
├──────────────────────────────────────────────────────────────────────────┤
│ 🐢 Idle in transaction (> 30s)                              [2 idle]    │
│   pid    user        idle_time    last_query                            │
│ ▸ 12348  worker      00:01:45.22  UPDATE jobs SET status=...            │
│   12349  app_user    00:00:34.10  SELECT * FROM users WHERE id=$1       │
├──────────────────────────────────────────────────────────────────────────┤
│ [q]uit  [r]efresh interval  [k]ill  [↑↓]navigate  [tab]section  [?]help │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Five core elements

| # | Element | Source | Notes |
|---|---|---|---|
| 1 | Header (connections / TPS / cache hit) | `pg_stat_database`, `pg_stat_activity` | TPS = delta of xact_commit + xact_rollback per interval |
| 2 | Long-running queries | `pg_stat_activity` | filter: state='active' AND duration > threshold |
| 3 | Locks | `pg_locks` JOIN `pg_stat_activity` | blocked-blocking pairs |
| 4 | Idle in transaction | `pg_stat_activity` | filter: state='idle in transaction' AND duration > threshold |
| 5 | Key bindings | (in-app) | `q`, `r`, `k`, arrows, tab, `?` |

### 2.3 Out of scope for v0.1 (deferred)

- Tabs / multi-view (v0.2 may add `pg_stat_statements` view)
- Historical trend graphs (v0.3)
- Config file editing, log tailing (v0.4)
- Replication lag (v0.4)
- Vacuum / autovacuum details (v0.5)
- Alerts (v0.6)
- Snapshot / replay (v1.0)
- Web UI (v2.0)

---

## 3. Architecture

### 3.1 Three-layer separation

Following the agreed direction: **complete separation between core, TUI, and (future) web**.

```
postgres-incident-tui/
├── cmd/
│   └── <name>/
│       └── main.go                # CLI entry, flag parsing, wires up tui
├── internal/
│   ├── core/                      # DB → Go structs (no formatting, no UI)
│   │   ├── client.go              # *pgx.Conn wrapper, connection lifecycle
│   │   ├── activity.go            # pg_stat_activity queries → []Activity
│   │   ├── locks.go               # pg_locks queries → []Lock
│   │   ├── stats.go               # pg_stat_database → DBStats
│   │   ├── poller.go              # Polling loop, snapshot diffing
│   │   └── types.go               # Activity, Lock, DBStats, Snapshot
│   ├── tui/                       # Bubble Tea Model/View/Update
│   │   ├── app.go                 # Root model, holds sub-models
│   │   ├── header.go
│   │   ├── activity_view.go
│   │   ├── locks_view.go
│   │   ├── idle_view.go
│   │   ├── keys.go                # Key bindings
│   │   ├── style.go               # Lipgloss styles
│   │   └── format.go              # Duration → "00:02:14.32" etc.
│   └── version/
│       └── version.go
├── e2e/                           # End-to-end tests against real PG
├── testdata/                      # Snapshot fixtures
├── .github/workflows/
├── Makefile
├── go.mod / go.sum
├── README.md
├── LICENSE
└── .goreleaser.yaml
```

### 3.2 Boundary rules

- **`core` knows nothing about the TUI.** No `lipgloss`, no Bubble Tea types, no formatting strings. It returns plain Go structs and `time.Duration`.
- **`tui` knows nothing about the SQL.** It receives structs from `core` and maps them to views.
- **`tui` does the formatting.** "00:02:14.32" string, table layout, colors — all in `tui`.
- **Both depend on `internal/version`.** Nothing else is shared.

When v1.0 adds a Web UI: a sibling `internal/web/` reuses `core` directly, with no changes needed in `core`.

### 3.3 Bubble Tea model shape (sketch)

```go
type App struct {
    poller   *core.Poller       // pushes snapshots via channel
    snapshot core.Snapshot      // last received

    section  Section            // current focused section (activity/locks/idle)
    cursor   map[Section]int    // selected row per section
    interval time.Duration      // refresh interval

    err      error              // last error (if any)
    quitting bool
}
```

`tea.Cmd` to fetch the next snapshot. The `core.Poller` does the work asynchronously and sends results back via a channel.

---

## 4. Data Model

Per the agreed direction: **only fields needed in v0.1**. Adding fields in v0.2+ is accepted as part of the cost.

### 4.1 Core types (in `internal/core/types.go`)

```go
// Snapshot is a single point-in-time capture of all dashboard data.
type Snapshot struct {
    CapturedAt    time.Time
    PGVersion     string         // e.g. "16.1"
    ServerAddr    string         // e.g. "10.0.1.42:5432"
    DBStats       DBStats
    Activities    []Activity     // long-running active queries
    Locks         []Lock         // waiting locks
    IdleInTx      []Activity     // idle in transaction sessions
}

// DBStats is the header row data.
type DBStats struct {
    ConnectionsActive int
    ConnectionsMax    int     // from `max_connections` setting
    TPS               float64 // computed from delta between snapshots
    CacheHitRatio     float64 // 0.0 – 1.0
}

// Activity represents one row from pg_stat_activity.
type Activity struct {
    PID         int
    User        string
    Database    string
    State       string         // "active" / "idle in transaction" / etc.
    QueryStart  time.Time
    Duration    time.Duration  // now() - query_start (or xact_start for idle-in-tx)
    Query       string         // truncated if track_activity_query_size is small
    Application string         // application_name
    Client      string         // client_addr or "(local)"
}

// Lock represents one blocked-blocking relationship.
type Lock struct {
    BlockedPID  int
    BlockingPID int
    WaitTime    time.Duration
    Relation    string         // schema.table, or empty if non-relation lock
    Mode        string         // "ShareLock", "ExclusiveLock", etc.
    LockType    string         // "relation", "transactionid", "tuple", etc.
}
```

### 4.2 Discarded for v0.1 (will add as needed)

- `xact_start`, `backend_start`, `state_change` (timestamps other than `query_start`)
- `wait_event`, `wait_event_type` (deferred to wait-event view in v0.3)
- `backend_type` (we filter to client backends only in v0.1)
- `pid` parent / leader info (parallel workers)

---

## 5. SQL Catalog (separate document)

> **This is the highest-risk area.** The exact SQL needs validation against PG 13/14/15/16 with `pg_monitor` privilege, on both vanilla PostgreSQL and RDS.

We will maintain `SQL_CATALOG.md` as a separate document, listing the candidate query for each metric, version notes, and verification status (✅ tested / ⚠️ untested / ❌ broken on PG X).

The DESIGN.md only documents *what* we query; the catalog documents *how*.

---

## 6. Update Loop

### 6.1 What needs to be decided

- **Polling cadence**: default 1 second. User can change with `+` / `-` keys (range 0.5 – 5.0s).
- **Snapshot diffing**: TPS and similar derived metrics need previous snapshot to compute delta.
- **Concurrency**: poller runs in its own goroutine, sends `Snapshot` to TUI via channel. TUI never blocks on DB.
- **Cancellation**: when user changes interval, in-flight query continues to completion; the next tick uses the new interval.
- **Error during poll**: error is delivered via the same channel as a separate variant. TUI displays it without crashing the loop.

### 6.2 Sketch

```go
type pollResult struct {
    snapshot core.Snapshot
    err      error
}

func (p *Poller) Run(ctx context.Context, out chan<- pollResult) {
    for {
        s, err := p.capture(ctx)
        select {
        case out <- pollResult{s, err}:
        case <-ctx.Done():
            return
        }
        select {
        case <-time.After(p.Interval()):  // Interval() reads atomically
        case <-ctx.Done():
            return
        }
    }
}
```

### 6.3 Open questions

- ⚠️ TPS calculation: does it use cumulative `xact_commit + xact_rollback` per database, summed? Or per-database? (Decide during SQL_CATALOG work.)
- ⚠️ First snapshot has no previous; TPS shows "—" until second tick. Cache hit ratio is cumulative since stats reset; show it as-is or compute delta? (User probably wants delta-since-last-tick, not since stats reset.)

---

## 7. Error Handling

### 7.1 What needs to be decided

There are roughly four error categories, each with a different UX:

| Category | Example | UX |
|---|---|---|
| **Startup error** | wrong DSN, can't connect | print error to stderr, exit 1 |
| **Permission error** | not member of `pg_monitor` | print explanation + how-to-grant, exit 2 |
| **Transient runtime error** | DB drops connection mid-poll | show banner "reconnecting…", auto-retry |
| **Fatal runtime error** | irrecoverable (rare) | show error, accept `q` to quit |

### 7.2 Where errors are shown in the TUI

A persistent **status bar** at the bottom (above key hints):

```
│ ⚠️ Lost connection. Retrying… (3s)                                       │
│ [q]uit  [r]efresh interval  [k]ill  [↑↓]navigate  [tab]section  [?]help │
```

Errors don't pop modals. Modals interrupt the workflow. Banner-style is consistent with k9s and lazygit.

### 7.3 Specific concerns

- **`pg_monitor` check at startup**: query `SELECT pg_has_role(current_user, 'pg_monitor', 'MEMBER')` and fail fast with an actionable error.
- **`pg_terminate_backend` requires more than `pg_monitor`**: actually requires `pg_signal_backend` role or higher. v0.1 should detect this and either disable the `[k]ill` key or show a permission error when invoked. *(This is a SQL_CATALOG question: exact role required.)*
- **Query truncation**: when `track_activity_query_size` is set low, `query` field is truncated. Show "…" indicator and document this in README.

---

## 8. Testing Strategy

This is one of the open questions. Options:

| Option | Approach | Pros | Cons |
|---|---|---|---|
| **a** | testcontainers-go (real PG) | high fidelity, catches version differences | slow, requires Docker, heavy CI |
| **b** | pgx mock | fast, deterministic | misses real PG behavior |
| **c** | hybrid: unit=mock, e2e=real PG | balanced | two test infrastructures |
| **d** | TUI snapshot tests (golden files) | UI regressions caught | doesn't test DB layer |

### 8.1 Recommendation

**c + d**, similar to colref / gomarklint:

- **Unit tests**: pure Go logic (formatters, snapshot diffing) — no DB, no TUI
- **Core integration tests**: testcontainers-go with real PG (one test per supported PG version: 13, 14, 15, 16, 17 — *configurable in CI matrix*)
- **TUI snapshot tests**: feed a fixed `core.Snapshot` to the TUI Model, render the View, compare to golden text file. Bubble Tea has `teatest` for this.
- **E2E**: spin up real PG, run the binary against it, verify output. Same shape as colref's e2e tests.

This matches the test infrastructure already in your other projects.

### 8.2 Open questions

- ⚠️ How many PG versions to test in CI? (cost vs coverage tradeoff)
- ⚠️ Do we test against RDS in CI? (probably not — too expensive, manual verification before release)

---

## 9. UX Details

### 9.1 Layout constraints

- **Minimum recommended size**: 120 columns × 40 rows.
- **Below minimum**: show a single-screen warning message ("Terminal too small. Resize to at least 120×40.") instead of trying to render a broken layout.
- **Above minimum**: scale section heights proportionally. Each section gets roughly 1/3 of the body area, with header (3 rows) and footer (2 rows) fixed.

### 9.2 Section navigation

- `Tab` cycles through sections (Long-running → Locks → Idle).
- Active section shows a colored left border or title prefix.
- `↑` / `↓` (or `j` / `k`) move within the active section.
- Selected row is highlighted.

### 9.3 Key bindings

| Key | Action |
|---|---|
| `q` / `Ctrl-C` | Quit |
| `?` | Help overlay (modal) |
| `Tab` | Next section |
| `Shift-Tab` | Previous section |
| `↑` / `↓` / `j` / `k` | Move cursor in active section |
| `+` / `-` | Increase / decrease refresh interval |
| `r` | Force refresh now |
| `k` | Kill (terminate) selected backend (with confirmation) |
| `c` | Cancel selected query (with confirmation) — softer than kill |
| `/` | Filter (deferred to v0.2; placeholder noted in help) |

### 9.4 Confirmation modals

For destructive actions (`k`, `c`):

```
┌─ Confirm ───────────────────────────────────┐
│                                             │
│  Terminate backend pid=12345?               │
│  Query: SELECT u.* FROM users u JOIN…       │
│                                             │
│  [y]es   [n]o                               │
└─────────────────────────────────────────────┘
```

---

## 10. Open Questions Summary

These are explicitly deferred to implementation phase:

| # | Question | Phase |
|---|---|---|
| OQ-1 | Exact SQL for each metric, per PG version | SQL_CATALOG validation |
| OQ-2 | TPS formula (per-db summed vs total) | SQL_CATALOG validation |
| OQ-3 | Cache hit: cumulative or delta? | SQL_CATALOG validation |
| OQ-4 | Permission level for `[k]ill` action | SQL_CATALOG validation |
| OQ-5 | Number of PG versions in CI matrix | Test infrastructure setup |
| OQ-6 | Project name | Last (after v0.1 features stabilize) |

---

## 11. Roadmap (post-v0.1)

| Version | Theme |
|---|---|
| v0.1 | Incident dashboard (this document) |
| v0.2 | `pg_stat_statements` integration (slow query history) |
| v0.3 | Trend graphs (last 5 minutes), wait events view |
| v0.4 | Replication monitoring, log tailing |
| v0.5 | Vacuum / autovacuum monitoring |
| v0.6 | Alerts (threshold-based, terminal notifications) |
| v1.0 | Snapshot save / replay; first stable release |
| v2.0 | Web UI mode (`<name> serve`) |

---

## 12. Things Intentionally NOT Decided Here

- **Project name** (per agreement: last)
- **License** (assume MIT, but confirm before public release)
- **Logo / branding** (post-v1.0 problem)
- **Homebrew formula, Docker image, etc.** (after v0.1 release works)

---

*End of v0.1 design.*
