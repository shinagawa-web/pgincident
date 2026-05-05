# pgincident

> "The first 30 seconds of a Postgres incident — in one screen."

A modern Postgres monitoring TUI focused on incident response. Inspired by `pgcenter` and `pg_activity`, but built with Bubble Tea and designed for managed PostgreSQL (RDS, Cloud SQL, Aurora) without requiring SUPERUSER.

---

## 1. Product Definition

### 1.1 Positioning

Targets SREs and Web engineers who reach for `psql -c "SELECT * FROM pg_stat_activity"` when production gets slow. Replaces a sequence of manual queries with a single live dashboard.

### 1.2 Differentiation from competitors

| | pgcenter | pg_activity | This project |
|---|---|---|---|
| Language | Go | Python | Go |
| TUI library | ncurses (gocui) | curses | **Bubble Tea** |
| OS | Linux only | Linux/Mac | **Linux + Mac** |
| Required privilege | SUPERUSER | SUPERUSER | **`pg_monitor`** |
| Managed DB (RDS, Cloud SQL) | partial | partial | **first-class** |
| Focus | comprehensive stats | top-style activity | **incident response + investigation** |
| Post-mortem export | no | no | **planned (v0.7)** |
| Web UI | no | no | **planned (v2.0)** |

The three big bets:

1. **`pg_monitor` instead of SUPERUSER** — unlocks managed PostgreSQL.
2. **Incident-response framing** — not "show me everything", but "what's broken right now."
3. **incident → investigate flow** — spot the symptom, drill into the root cause without leaving the terminal.

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
┌─ pgincident v0.1.0 ───────── connected: prod-db@10.0.1.42:5432 (PG 16.1) ─┐
│ Connections: 142 / 200 (71%)   TPS: 2,340   Cache hit: 99.2%               │
├─────────────────────────────────────────────────────────────────────────────┤
│ Long-running queries (> 5s)                                    [12 active]  │
│   PID     USER           DURATION      STATE        QUERY                   │
│ ▶ 12345  app_user    00:02:14.32  active   SELECT u.* FROM users u JOIN…    │
│   12346  worker      00:00:18.04  active   UPDATE jobs SET status=...       │
├─────────────────────────────────────────────────────────────────────────────┤
│ Locks (waiting)                                                 [3 waiting] │
│   BLOCKED  BLOCKING   WAIT TIME     RELATION             MODE               │
│ ▶ 12350    12345     00:01:23.10  public.users   ShareLock                  │
├─────────────────────────────────────────────────────────────────────────────┤
│ Idle in transaction (> 30s)                                      [2 idle]   │
│   PID     USER           IDLE TIME     LAST QUERY                           │
│ ▶ 12348  worker      00:01:45.22  UPDATE jobs SET status=...                │
├─────────────────────────────────────────────────────────────────────────────┤
│ [q]uit  [Tab]section  [↑↓/jk]cursor  [+/-]interval  [?]help                │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Five core elements

| # | Element | Source | Notes |
|---|---|---|---|
| 1 | Header (connections / TPS / cache hit) | `pg_stat_database`, `pg_stat_activity` | TPS = delta of xact_commit + xact_rollback per interval |
| 2 | Long-running queries | `pg_stat_activity` | filter: state='active' AND duration > threshold (default 5s) |
| 3 | Locks | `pg_locks` JOIN `pg_stat_activity` | blocked-blocking pairs |
| 4 | Idle in transaction | `pg_stat_activity` | filter: state='idle in transaction' AND duration > threshold (default 30s) |
| 5 | Key bindings | (in-app) | `q`, `Tab`, `↑↓/jk`, `+/-`, `?` |

### 2.3 Out of scope for v0.1 (deferred)

- `pg_stat_statements` integration (v0.2)
- Investigate mode / drill-down (v0.3)
- Replication monitoring, log tailing (v0.4)
- Snapshot recording (v0.5)
- Autovacuum / wraparound detection (v0.6)
- Post-mortem export (v0.7)
- Alerts (v0.8)
- Snapshot replay, Azure/Neon/Supabase (v1.0)
- Web UI (v2.0)

---

## 3. Architecture

### 3.1 Three-layer separation

```
pgincident/
├── cmd/
│   └── pgincident/
│       └── main.go                # CLI entry, wires up core + tui
├── internal/
│   ├── core/                      # DB → Go structs (no formatting, no UI)
│   │   ├── client.go              # pgx connection wrapper
│   │   ├── activity.go            # pg_stat_activity → []Activity
│   │   ├── locks.go               # pg_locks → []Lock
│   │   ├── stats.go               # pg_stat_database → DBStats
│   │   ├── poller.go              # background polling loop
│   │   └── types.go               # Activity, Lock, DBStats, Snapshot
│   ├── tui/                       # Bubble Tea Model/View/Update
│   │   ├── app.go                 # root model
│   │   ├── header.go
│   │   ├── activity_view.go
│   │   ├── locks_view.go
│   │   ├── idle_view.go
│   │   ├── section.go
│   │   ├── style.go               # Lipgloss styles
│   │   └── format.go              # duration / padding helpers
│   └── version/
│       └── version.go
├── .github/workflows/
├── Makefile
├── go.mod / go.sum
└── README.md
```

### 3.2 Boundary rules

- **`core` knows nothing about the TUI.** No lipgloss, no Bubble Tea types, no formatting. Returns plain Go structs and `time.Duration`.
- **`tui` knows nothing about the SQL.** Receives structs from `core` and maps them to views.
- **Both depend on `internal/version`.** Nothing else is shared.

---

## 4. Data Model

### 4.1 Core types (`internal/core/types.go`)

```go
type Snapshot struct {
    CapturedAt time.Time
    PGVersion  string
    ServerAddr string
    DBStats    DBStats
    Activities []Activity // long-running active queries
    Locks      []Lock     // waiting lock pairs
    IdleInTx   []Activity // idle in transaction sessions
}

type DBStats struct {
    ConnectionsActive int
    ConnectionsMax    int
    TPS               float64 // delta between snapshots
    CacheHitRatio     float64 // 0.0 – 1.0, cumulative since stats reset
    XactTotal         int64
}

type Activity struct {
    PID         int
    User        string
    Database    string
    State       string
    QueryStart  time.Time
    Duration    time.Duration
    Query       string
    Application string
    Client      string
}

type Lock struct {
    BlockedPID  int
    BlockingPID int
    WaitTime    time.Duration
    Relation    string
    Mode        string
    LockType    string
}
```

---

## 5. SQL Catalog

See `SQL_CATALOG.md` for the candidate SQL per metric, version notes, and verification status (✅ tested / ⚠️ untested / ❌ broken on PG X).

---

## 6. Update Loop

- Default interval: 1 second. Adjustable with `+` / `-` (minimum 100ms).
- Poller runs in a background goroutine, sends `PollResult` to TUI via channel. TUI never blocks on DB.
- Uses `time.NewTimer` (not `time.After`) to avoid timer leaks.
- TPS skipped when `XactTotal` goes backward (server restart / `pg_stat_reset`).

---

## 7. Error Handling

| Category | Example | UX |
|---|---|---|
| Startup error | wrong DSN, can't connect | print to stderr, exit 1 |
| Permission error | not member of `pg_monitor` | print explanation + grant command, exit 1 |
| Transient runtime error | lost connection mid-poll | error banner in status bar |

`pg_monitor` membership is checked at startup. If the user is not a member, the tool exits with an actionable message.

---

## 8. Testing

See issue #10 for v0.1.1 scope. Strategy:

- **Unit tests**: pure Go logic (formatters, poller math) — no DB
- **Integration tests**: real Postgres via `DATABASE_URL`; skipped if unreachable
- **CI**: GitHub Actions with Postgres 16 service container

---

## 9. UX Details

### 9.1 Two-mode design

- **`incident` mode** (default) — "what's broken right now?" Seconds-scale polling. Always the entry point.
- **`investigate` mode** — drill into a selected row (v0.3, via `Enter`).

### 9.2 Layout constraints

- **Minimum supported size**: 80 columns × 24 rows.
- Below minimum: warning screen instead of broken layout.
- Above minimum: each section gets roughly 1/3 of the body area.

### 9.3 Key bindings

| Key | Action |
|---|---|
| `q` / `Ctrl-C` | Quit |
| `?` | Help overlay |
| `Tab` | Next section |
| `Shift-Tab` | Previous section |
| `↑` / `↓` / `j` / `k` | Move cursor in active section |
| `Enter` | Investigate mode for selected row (v0.3+) |
| `+` / `-` | Increase / decrease refresh interval |

---

## 10. Roadmap

| Version | Theme |
|---|---|
| ~~v0.1~~ | ~~Incident dashboard~~ ✅ |
| v0.1.1 | Unit tests + integration tests + CI |
| v0.2 | `pg_stat_statements` integration (slow query history); RDS verification |
| v0.3 | Investigate mode — index health, bloat, wait events (via `Enter`) |
| v0.4 | Replication monitoring, log tailing |
| v0.5 | Snapshot recording (local SQLite) |
| v0.6 | Autovacuum / wraparound danger detection |
| v0.7 | Post-mortem auto-generation (Markdown export) |
| v0.8 | Alerts (threshold-based, terminal notifications) |
| v1.0 | Snapshot save / replay; Azure / Neon / Supabase support; stable release |
| v2.0 | Web UI mode (`pgincident serve`) |

**Design decisions:**

- Opened during incidents, not as a routine monitor. No standalone health dashboard.
- APM / OpenTelemetry integration is out of scope.
- Platform support beyond RDS / Cloud SQL deferred to v1.0.
- Post-mortem generation (v0.7) depends on snapshot recording (v0.5).

---

## Development

### Prerequisites

- Go 1.22+
- Docker (for local Postgres)

### Start the dev environment

```bash
make dev-up   # starts Postgres 16 with pgincident_dev / pg_monitor
make run      # builds and launches the TUI
make dev-down # stop and remove the container
```

Default DSN (used when `DATABASE_URL` is not set):
```
postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres
```

### Simulating incident scenarios

Each scenario requires separate terminal windows.

**Long-running query (> 5s)**

Terminal 1 — seed a 30-second sleep:
```bash
docker compose exec postgres psql -U postgres -c "SELECT pg_sleep(30);"
```
Terminal 2 — run the TUI and verify section 1 shows the query:
```bash
make run
```

**Lock (blocked / blocking)**

Terminal 1 — hold an exclusive lock:
```bash
docker compose exec postgres psql -U postgres -c \
  "BEGIN; LOCK TABLE seed_target IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(60);"
```
Terminal 2 — issue a blocked query:
```bash
docker compose exec postgres psql -U postgres -c "SELECT * FROM seed_target;"
```
Terminal 3 — run the TUI and verify section 2 shows the lock pair:
```bash
make run
```

**Idle in transaction (> 30s)**

Terminal 1 — open an interactive session and leave a transaction open:
```bash
docker compose exec -it postgres psql -U postgres
```
Inside psql:
```sql
BEGIN;
SELECT 1;
-- do not COMMIT or ROLLBACK — leave the session open
```
After 30 seconds, run the TUI and verify section 3 shows the idle session:
```bash
make run
```
