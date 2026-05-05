# SQL Catalog

Verification status: ✅ tested / ⚠️ untested / ❌ broken on PG X

Required privilege: `pg_monitor` (no SUPERUSER)

---

## 1. pg_monitor membership check

```sql
SELECT pg_has_role(current_user, 'pg_monitor', 'MEMBER')
```

| PG 13 | PG 14 | PG 15 | PG 16 | PG 17 | RDS | Cloud SQL |
|-------|-------|-------|-------|-------|-----|-----------|
| ⚠️ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ |

---

## 2. Long-running active queries

```sql
SELECT
    pid,
    COALESCE(usename, ''),
    COALESCE(datname, ''),
    COALESCE(state, ''),
    COALESCE(query_start, now()),
    now() - COALESCE(query_start, now()),
    left(COALESCE(query, ''), 200),
    COALESCE(application_name, ''),
    COALESCE(client_addr::text, '(local)')
FROM pg_stat_activity
WHERE state = 'active'
  AND pid <> pg_backend_pid()
  AND query_start < now() - ($1 * interval '1 second')
ORDER BY query_start
```

Notes:
- `pg_stat_activity` is readable by `pg_monitor` since PG 10.
- `query` field is truncated by `track_activity_query_size` (default 1024 bytes). Show "…" if truncated.
- `client_addr` is NULL for local (Unix socket) connections.

| PG 13 | PG 14 | PG 15 | PG 16 | PG 17 | RDS | Cloud SQL |
|-------|-------|-------|-------|-------|-----|-----------|
| ⚠️ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ |

---

## 3. Waiting locks

```sql
SELECT
    blocked.pid,
    blocking.pid,
    now() - blocked.query_start,
    COALESCE(relation::regclass::text, ''),
    blocked_locks.mode,
    blocked_locks.locktype
FROM pg_locks blocked_locks
JOIN pg_stat_activity blocked ON blocked.pid = blocked_locks.pid
JOIN pg_locks blocking_locks
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid <> blocked_locks.pid
JOIN pg_stat_activity blocking ON blocking.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted
ORDER BY blocked.query_start
```

Notes:
- `pg_locks` is readable by `pg_monitor` since PG 10.
- `relation::regclass` may fail if the relation is dropped mid-query — wrap in COALESCE.

| PG 13 | PG 14 | PG 15 | PG 16 | PG 17 | RDS | Cloud SQL |
|-------|-------|-------|-------|-------|-----|-----------|
| ⚠️ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ |

---

## 4. Idle in transaction

```sql
SELECT
    pid,
    COALESCE(usename, ''),
    COALESCE(datname, ''),
    state,
    COALESCE(xact_start, now()),
    now() - COALESCE(xact_start, now()),
    left(COALESCE(query, ''), 200),
    COALESCE(application_name, ''),
    COALESCE(client_addr::text, '(local)')
FROM pg_stat_activity
WHERE state = 'idle in transaction'
  AND xact_start < now() - ($1 * interval '1 second')
ORDER BY xact_start
```

Notes:
- Uses `xact_start` (not `query_start`) — measures how long the transaction has been open.

| PG 13 | PG 14 | PG 15 | PG 16 | PG 17 | RDS | Cloud SQL |
|-------|-------|-------|-------|-------|-----|-----------|
| ⚠️ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ |

---

## 5. Header stats (connections / TPS / cache hit)

```sql
SELECT
    count(*) FILTER (WHERE state IS NOT NULL)    AS connections_active,
    (SELECT setting::int FROM pg_settings WHERE name = 'max_connections') AS connections_max,
    sum(xact_commit + xact_rollback)             AS xact_total,
    sum(blks_hit)::float / NULLIF(sum(blks_hit + blks_read), 0) AS cache_hit_ratio
FROM pg_stat_activity, pg_stat_database
WHERE pg_stat_database.datname = current_database()
GROUP BY 1
```

Notes:
- TPS = delta of `xact_total` between snapshots / interval seconds.
- Cache hit ratio: cumulative since last stats reset.

| PG 13 | PG 14 | PG 15 | PG 16 | PG 17 | RDS | Cloud SQL |
|-------|-------|-------|-------|-------|-----|-----------|
| ⚠️ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ |
