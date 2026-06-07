---
title: Incident scenarios
weight: 20
---

# Incident scenarios

Manual steps to trigger each dashboard section in the dev environment. Run `make dev-up` first.

## Long-running query (> 5 s)

Terminal 1 — run a query that sleeps for 60 s:

```sh
docker compose exec -T postgres psql -U postgres -c \
  "SELECT pg_sleep(60);"
```

Terminal 2 — run the TUI and verify the Long-running queries section shows the row:

```sh
make run
```

Press `↓`/`j` to select the row, then `Enter` to open the query detail overlay. Press `↑`/`↓` to scroll.

## Lock contention

Terminal 1 — hold an exclusive lock:

```sh
docker compose exec postgres psql -U postgres -c \
  "BEGIN; LOCK TABLE seed_target IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(60);"
```

Terminal 2 — issue a blocked query:

```sh
docker compose exec postgres psql -U postgres -c "SELECT * FROM seed_target;"
```

Terminal 3 — run the TUI and verify the Locks section shows the blocked/blocking pair:

```sh
make run
```

## Idle in transaction (> 30 s)

Terminal 1 — open an interactive session and leave a transaction open:

```sh
docker compose exec -it postgres psql -U postgres
```

Inside psql:

```sql
BEGIN;
SELECT 1;
-- do not COMMIT or ROLLBACK
```

After 30 seconds, run the TUI and verify the Idle in transaction section shows the session:

```sh
make run
```
