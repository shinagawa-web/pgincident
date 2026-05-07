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

Terminal 1 — run a wide diagnostic query that blocks for 60 s and formats to
more lines than a typical terminal height so you can test scrolling in the detail overlay:
```bash
docker compose exec -T postgres psql -U postgres << 'EOF'
WITH paused AS (SELECT pg_sleep(60))
SELECT a.pid, a.usename, a.application_name, a.client_addr, a.client_hostname,
       a.client_port, a.backend_start, a.xact_start, a.query_start, a.state_change,
       a.wait_event_type, a.wait_event, a.state, a.backend_xid, a.backend_xmin,
       a.query, a.backend_type, a.leader_pid,
       l.locktype, l.database, l.relation::regclass AS locked_table, l.page, l.tuple,
       l.virtualxid, l.transactionid, l.classid, l.objid, l.objsubid,
       l.virtualtransaction, l.pid AS lock_pid, l.mode, l.granted, l.fastpath,
       s.seq_scan, s.seq_tup_read, s.idx_scan, s.idx_tup_fetch,
       s.n_tup_ins, s.n_tup_upd, s.n_tup_del, s.n_tup_hot_upd,
       s.n_live_tup, s.n_dead_tup, s.n_mod_since_analyze,
       s.last_vacuum, s.last_autovacuum, s.last_analyze, s.last_autoanalyze,
       ui.indexrelname, ui.idx_scan AS idx_idx_scan, ui.idx_tup_read,
       ui.idx_tup_fetch AS idx_tup_fetch2,
       bg.checkpoints_timed, bg.checkpoints_req,
       bg.checkpoint_write_time, bg.checkpoint_sync_time,
       bg.buffers_checkpoint, bg.buffers_clean, bg.maxwritten_clean,
       bg.buffers_backend, bg.buffers_backend_fsync, bg.buffers_alloc,
       bg.stats_reset AS bg_stats_reset,
       r.usesysid AS repl_usesysid, r.usename AS repl_usename,
       r.application_name AS repl_app_name, r.client_addr AS repl_client_addr,
       r.state AS repl_state, r.sent_lsn, r.write_lsn, r.flush_lsn, r.replay_lsn,
       r.write_lag, r.flush_lag, r.replay_lag, r.sync_priority, r.sync_state,
       ssl.ssl, ssl.version AS ssl_version, ssl.cipher AS ssl_cipher, ssl.bits AS ssl_bits,
       now() - a.query_start AS query_duration,
       now() - a.xact_start AS xact_duration,
       now() - a.state_change AS time_in_state,
       now() - a.backend_start AS connection_age
FROM paused, pg_stat_activity a
JOIN pg_locks l ON l.pid = a.pid AND l.granted = false
JOIN pg_stat_user_tables s ON s.relid = l.relation
JOIN pg_stat_bgwriter bg ON TRUE
LEFT JOIN pg_stat_replication r ON r.active_pid = a.pid
LEFT JOIN pg_stat_ssl ssl ON ssl.pid = a.pid
LEFT JOIN pg_stat_user_indexes ui ON ui.relid = s.relid
LEFT JOIN pg_stat_wal_receiver wr ON TRUE
LEFT JOIN pg_stat_archiver arch ON TRUE
WHERE a.state != 'idle' AND a.pid != pg_backend_pid()
ORDER BY query_duration DESC
LIMIT 50;
EOF
```
Terminal 2 — run the TUI, select the row with `↓/j`, press `Enter` to open the detail
overlay, then press `↑/k` or `↓/j` to scroll through the SQL:
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
