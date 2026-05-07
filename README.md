# pgincident

[![CI](https://github.com/shinagawa-web/pgincident/actions/workflows/ci.yml/badge.svg)](https://github.com/shinagawa-web/pgincident/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/shinagawa-web/pgincident/branch/main/graph/badge.svg)](https://codecov.io/gh/shinagawa-web/pgincident)

> "The first 30 seconds of a Postgres incident — in one terminal."

Production Postgres is slow. You open psql and start firing queries — `pg_stat_activity`, `pg_locks`, `pg_stat_statements` — each in a separate window, refreshed by hand. By the time you've pieced together what's happening, the incident is already a minute old.

**pgincident** collapses that into a single live dashboard.

## 1. Product Definition

### 1.1 Positioning

Targets SREs and Web engineers who reach for `psql -c "SELECT * FROM pg_stat_activity"` when production gets slow. Replaces a sequence of manual queries with a single live dashboard.

### 1.2 Non-goals (v0.1)

- Replacing pgAdmin / DBeaver (no schema browsing, no query editor)
- Long-term metrics storage (Prometheus, Grafana already do this)
- Replication monitoring (later version)
- System stats (CPU/IO/mem) — pgcenter does this; we focus on Postgres internals
- Multi-instance dashboard (one connection at a time in v0.1)

## 2. v0.1 Feature Scope

### 2.1 Single-screen Incident Dashboard

```
pgincident v0.1.0              connected: 10.0.1.42:5432 (PG 16.1)  interval: 5.0s
Connections: 142/200 (71%)   TPS: 2340   Cache hit: 99.2%
─────────────────────────────────────────────────────────────────────────────────
Long-running queries (> 5s)                                         [12 active]
  PID     USER           DURATION      STATE        QUERY
▸ 12345  app_user    00:02:14.32  active   SELECT u.* FROM users u JOIN…
  12346  worker      00:00:18.04  active   UPDATE jobs SET status=...
─────────────────────────────────────────────────────────────────────────────────
Locks (waiting)                                                      [3 waiting]
  BLOCKED  BLOCKING   WAIT TIME     RELATION             MODE
  12350    12345     00:01:23.10  public.users   ShareLock
─────────────────────────────────────────────────────────────────────────────────
Idle in transaction (> 30s)                                            [2 idle]
  PID     USER           IDLE TIME     LAST QUERY
  12348  worker      00:01:45.22  UPDATE jobs SET status=...
─────────────────────────────────────────────────────────────────────────────────
[q]uit  [Tab]section  [↑↓/jk]cursor  [+/-]interval  [?]help
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
- Snapshot replay, Azure/Neon/Supabase (v1.0)
- Web UI (v2.0)

## 5. SQL Catalog

See `SQL_CATALOG.md` for the candidate SQL per metric, version notes, and verification status (✅ tested / ⚠️ untested / ❌ broken on PG X).

## 6. Update Loop

- Default interval: 5 seconds. Adjustable with `+` / `-` (minimum 1s).
- Poller runs in a background goroutine, sends `PollResult` to TUI via channel. TUI never blocks on DB.
- Uses `time.NewTimer` (not `time.After`) to avoid timer leaks.
- TPS skipped when `XactTotal` goes backward (server restart / `pg_stat_reset`).

### DB load

All polled views (`pg_stat_activity`, `pg_locks`, `pg_stat_database`) read from shared memory with no disk I/O. Each query typically completes in < 1ms; total overhead is a few ms/s with negligible CPU impact (< 0.1%). A single persistent connection is reused — no per-poll connection cost.

Note: `pg_stat_statements` (v0.2) can be heavier on systems with many unique queries. Consider polling it at a longer interval or making it opt-in.

## 7. Error Handling

| Category | Example | UX |
|---|---|---|
| Startup error | wrong DSN, can't connect | print to stderr, exit 1 |
| Permission error | not member of `pg_monitor` | print explanation + grant command, exit 1 |
| Transient runtime error | lost connection mid-poll | error banner in status bar |

`pg_monitor` membership is checked at startup. If the user is not a member, the tool exits with an actionable message.

## 8. Testing

Planned for v0.1.1 (see issue [#10](https://github.com/shinagawa-web/pgincident/issues/10)). Strategy:

- **Unit tests**: pure Go logic (formatters, poller math) — no DB
- **Integration tests**: real Postgres via `DATABASE_URL`; skipped if unreachable
- **CI**: GitHub Actions with Postgres 16 service container

## 9. UX Details

### 9.1 Three-level design (target architecture)

> v0.1 ships a single dashboard screen (Level 2 entry point). Level 1 overview is planned for v0.1.3; full Level 3 investigation for v0.3.

- **Level 1 — Overview** (v0.1.3) — Global DB health at a glance. Key metrics with status colors (normal / warning / critical). If something is red, drill into Level 2.
- **Level 2 — Category view** — Per-category lists: Activity / Locks / I/O / Statements / Tables / Vacuum / Replication / Connections. *(v0.1 ships Activity, Locks, Idle in transaction)*
- **Level 3 — Process view** (`Enter`, v0.3+) — Individual query and session investigation. Full SQL, wait events, lock chain, cancel/kill.

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

## Why pgincident?

| | pgcenter | pg_activity | pgincident |
|---|---|---|---|
| Language | Go | Python | Go |
| OS | Linux only | Linux/Mac | Linux + Mac |
| Required privilege | SUPERUSER | SUPERUSER | **`pg_monitor`** |
| Managed DB (RDS, Cloud SQL) | partial | partial | **first-class** |
| Focus | comprehensive stats | top-style activity | **incident response + investigation** |
| Post-mortem export | no | no | planned (v0.7) |

Three key decisions behind this tool:

1. **`pg_monitor` instead of SUPERUSER** — unlocks managed PostgreSQL (RDS, Cloud SQL, Aurora).
2. **Incident-response framing** — not "show me everything", but "what's broken right now."
3. **overview → category → process flow** — global health first, drill into the problem area, then individual session investigation.

## 10. Roadmap

- [x] v0.1 — Incident dashboard
- [x] v0.1.1 — Unit tests + integration tests + CI
- [x] v0.1.2 — Query detail overlay (full SQL on `Enter`)
- [ ] v0.1.3 — Level 1 overview screen — global health indicators with status colors (connections, TPS, cache hit, checkpoint, replication lag, autovacuum)
- [ ] v0.2 — `pg_stat_statements` integration (slow query history); RDS verification
- [ ] v0.2.1 — Config file (`~/.pgincident.yml`) — slow query / idle-in-transaction thresholds, default DSN, refresh interval
- [ ] v0.2.2 — Connection presets in config; in-TUI connection switching
- [ ] v0.3 — Investigate mode — index health, bloat, wait events, seq scan / dead tuple ratio, checkpoint / bgwriter stats, WAL stats (via `Enter`); cancel (`c`) / kill (`K`) selected session (`pg_signal_backend` required)
- [ ] v0.4 — Replication monitoring, log tailing
- [ ] v0.5 — Snapshot recording + replay (local SQLite)
- [ ] v0.5.1 — PgBouncer stats integration
- [ ] v0.6 — Autovacuum / wraparound danger detection
- [ ] v0.7 — Post-mortem auto-generation (Markdown export)
- [ ] v0.8 — Multi-instance support — writer/replica simultaneous view (tab switching or fleet view TBD)
- [ ] v0.9 — Built-in SSH tunnel (bastion host support via config)
- [ ] v1.0 — Azure / Neon / Supabase support; stable release
- [ ] v2.0 — Web UI mode (`pgincident serve`)

## Development

### Architecture

Three-layer separation:

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
├── .github/workflows/             # planned in v0.1.1
├── Makefile
├── go.mod / go.sum
└── README.md
```

Boundary rules:

- **`core` knows nothing about the TUI.** No lipgloss, no Bubble Tea types, no formatting. Returns plain Go structs and `time.Duration`.
- **`tui` knows nothing about the SQL.** Receives structs from `core` and maps them to views.
- **Only `tui` depends on `internal/version`.** Nothing else is shared.

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to simulate incident scenarios locally.
