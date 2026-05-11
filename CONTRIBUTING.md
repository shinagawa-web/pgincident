# Contributing

## Dev environment

```bash
make dev-up   # starts Postgres 16 with pgincident_dev / pg_monitor
make run      # builds and launches the TUI
make dev-down # stop and remove the container
```

## Pre-push hook

Install the hook once after cloning:

```bash
make install-hooks
```

The hook runs automatically on every `git push` and checks:

- `go mod tidy` — fails if `go.mod`/`go.sum` are out of sync
- `go build ./...`
- `go vet ./...`
- Unit + e2e tests (`go test -race ./internal/...`)
- Integration tests (skipped automatically when `DATABASE_URL` is unset)

Bypass with `git push --no-verify` if needed.

## Useful make targets

| Target | Description |
|---|---|
| `make test` | Run all unit and e2e tests with race detector (writes `coverage.out`) |
| `make test-integration` | Run integration tests (requires `DATABASE_URL`) |
| `make lint` | Run `go vet` |
| `make tidy` | Run `go mod tidy` and verify no diff |
| `make install-hooks` | Install the pre-push hook |
| `make dev-load` | Start the workload simulator against the dev DB (see below) |

## End-to-end dashboard evaluation with the workload simulator

`make dev-load` is the default way to evaluate the dashboard as a whole under realistic,
production-like conditions.

```bash
# Terminal 1 — start the simulator (Ctrl-C to stop cleanly)
make dev-load

# Terminal 2 — run the TUI alongside it
make run
```

Within roughly 10 seconds all four dashboard sections should show non-empty, time-varying
data. The "Idle in transaction" section takes ~30 s to populate (that is the detection
threshold).

> **First run on an existing container:** `make dev-load` runs `dev/loadgen_setup.sql`
> automatically, which creates and seeds the `loadgen_accounts` and `loadgen_lock_rows`
> tables. No manual steps are needed. If you started fresh with `make dev-up` the tables
> are already present via `dev/init.sql`.

### Toggling subsystems

All four subsystems are enabled by default. Disable individual ones via `LOADGEN_FLAGS`:

```bash
# Only background TPS (no slow queries, no locks, no idle sessions)
make dev-load LOADGEN_FLAGS="--no-slow --no-locks --no-idle"

# Only lock contention
make dev-load LOADGEN_FLAGS="--no-tps --no-slow --no-idle"
```

Available flags for `go run ./cmd/pgincident-loadgen`:

| Flag | Default | Description |
|---|---|---|
| `--no-tps` | false | Disable background TPS workers |
| `--no-slow` | false | Disable slow-query workers |
| `--no-locks` | false | Disable lock contention workers |
| `--no-idle` | false | Disable idle-in-transaction workers |
| `--tps-workers N` | 8 | Number of background TPS connections |
| `--slow-interval D` | 6s | Delay between slow queries per worker |
| `--idle-duration D` | 45s | How long each idle-in-transaction session lingers |
| `--dsn URL` | env `DATABASE_URL` | PostgreSQL DSN |

### Connection budget

The simulator opens at most ~15 connections by default
(8 TPS pool + 2 slow + 2 lock holder/waiter + 2 idle).
Adjust `--tps-workers` if your dev Postgres has a lower `max_connections`.

## Simulating incident scenarios

Each scenario requires separate terminal windows.

**Long-running query (> 5s)**

Terminal 1 — run a realistic multi-clause query that blocks for 60 s:
```bash
docker compose exec -T postgres psql -U postgres << 'EOF'
WITH paused AS (SELECT pg_sleep(60))
SELECT a.pid, a.usename, a.state, l.relation::regclass AS locked_relation, l.mode
FROM paused, pg_locks l
JOIN pg_stat_activity a ON a.pid = l.pid
WHERE l.granted = true
ORDER BY a.query_start;
EOF
```
Terminal 2 — run the TUI, select the row with `↓/j` and press `Enter` to verify the
detail overlay shows the query formatted with clause breaks and keyword highlighting:
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
