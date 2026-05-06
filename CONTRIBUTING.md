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

## Simulating incident scenarios

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
