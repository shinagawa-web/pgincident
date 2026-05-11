# dev-load — workload simulator

`make dev-load` spins up a realistic, sustained workload against the dev Postgres so the dashboard can be evaluated end-to-end under production-like conditions.

## How it works

```
make dev-load
  │
  ├─ docker compose exec postgres psql -U postgres < dev/loadgen_setup.sql
  │     Creates loadgen_accounts (10 k rows) and loadgen_lock_rows (2 rows)
  │     and grants SELECT/UPDATE to pgincident_dev. Idempotent — safe to run
  │     against an existing container.
  │
  └─ go run ./cmd/pgincident-loadgen
        Launches four subsystems as goroutines and blocks until Ctrl-C.
```

## Subsystems

Each subsystem runs **short-lived** and **long-lived** sessions concurrently from startup so the dashboard always shows both quickly-resolving events and sessions that linger for minutes.

### TPS (background OLTP)

8 connections share a `pgxpool` and loop tight SELECT/UPDATE queries against `loadgen_accounts`. This drives the **Connections**, **TPS**, and **Cache hit** header metrics.

### Slow queries

| Worker | Query duration | Gap between runs |
|---|---|---|
| Short × 2 | 6–11 s (above the 5 s threshold) | 10 s, staggered 5 s apart |
| Long × 2 | 5–8 min | 30 s, staggered ~3 min apart |

The two long workers are offset by half the average query duration (~3.25 min). This ensures at least one long query is always visible in the dashboard, with an overlap window where two appear simultaneously.

```
time  0      3m     6m     9m    12m
      ├──────┤      ├──────┤
              ├──────┤      ├──────┤
```

Each worker holds its own `pgx.Conn` so query state is never shared across goroutines.

### Lock contention

Two independent lock cycles run on separate rows of `loadgen_lock_rows` so they do not interfere with each other or with the single-shot psql recipes in CONTRIBUTING.md.

| Cycle | Row | Hold duration | Pause between cycles |
|---|---|---|---|
| Short | row 1 | 8–18 s | 5 s |
| Long | row 2 | 5–8 min | 30 s |

Each cycle uses two separate `pgx.Conn` instances — a **holder** and a **waiter**:

1. Holder opens `BEGIN` and acquires `SELECT … FOR UPDATE` on the target row.
2. Waiter opens `BEGIN` and issues the same `SELECT … FOR UPDATE` — blocks immediately.
3. Both the blocking and the blocked session appear in the dashboard's **Locks (waiting)** section. The holder also appears in **Idle in transaction** (it is in an open transaction but not running a query).
4. After the hold duration, holder issues `ROLLBACK`. The waiter unblocks, commits, and the pair disappears from the dashboard.

The waiter goroutine is always drained before `lockCycle` returns to prevent a data race: `pgx.Conn` is not goroutine-safe, and the deferred `ROLLBACK` on the waiter connection must not run concurrently with its in-flight `Exec`.

### Idle in transaction

| Worker | Idle duration | Initial delay |
|---|---|---|
| Short × 2 | 45 s | 0 s and 22 s |
| Long × 1 | 6–8 min | 0 s |

The two short workers are staggered by 22 s (half of 45 s) so at least one is always past the 30 s detection threshold. The long worker provides a session that stays visible for minutes.

After each idle period the session randomly commits or rolls back, waits 2 s, then opens a new transaction and repeats.

## Graceful shutdown

Ctrl-C sends SIGINT → context is cancelled → every goroutine exits its loop → each connection sends `ROLLBACK` via `context.Background()` (so the rollback goes through even though the parent context is already cancelled) → `sync.WaitGroup` blocks until all goroutines finish → process exits.

After a clean shutdown, `pg_stat_activity` returns no `pgincident_dev` rows and `pg_locks WHERE NOT granted` is empty.

## Connection budget

| Subsystem | Connections |
|---|---|
| TPS pool | 8 (configurable via `--tps-workers`) |
| Slow short | 2 |
| Slow long | 2 |
| Lock short (holder + waiter) | 2 |
| Lock long (holder + waiter) | 2 |
| Idle short | 2 |
| Idle long | 1 |
| **Total** | **~19** |

Postgres default `max_connections` is 100, so the simulator is well within budget. Reduce `--tps-workers` if your environment has a lower limit.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--no-tps` | false | Disable TPS workers |
| `--no-slow` | false | Disable slow-query workers |
| `--no-locks` | false | Disable lock contention workers |
| `--no-idle` | false | Disable idle-in-transaction workers |
| `--tps-workers N` | 8 | Number of TPS pool connections |
| `--dsn URL` | `$DATABASE_URL` or default dev DSN | PostgreSQL connection string |

Pass flags via `LOADGEN_FLAGS`:

```bash
# Only lock contention
make dev-load LOADGEN_FLAGS="--no-tps --no-slow --no-idle"
```
