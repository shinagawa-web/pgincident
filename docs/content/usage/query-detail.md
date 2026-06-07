---
title: Query detail overlay
weight: 30
---

# Query detail overlay

Press `Enter` on a row in the Long-running queries section to open the full SQL. The query is formatted with clause breaks and keyword highlighting.

```
┌─ Query Detail ──────────────────────────────────────────────────────────┐
│ PID: 12345   user: app_user   duration: 00:02:14.32   state: active     │
│ ─────────────────────────────────────────────────────────────────────── │
│ SELECT                                                                   │
│   u.id, u.name, u.email,                                                │
│   o.id AS order_id, o.status, o.total_amount                            │
│ FROM users u                                                             │
│ JOIN orders o ON o.user_id = u.id                                       │
│ WHERE u.status = 'active'                                               │
│   AND o.created_at > NOW() - INTERVAL '7 days'                         │
│ ORDER BY o.created_at DESC                                              │
│ LIMIT 100                                                               │
└─────────────────────────────────────────────────────────────────────────┘
[any key] close
```

Press `↑`/`k` or `↓`/`j` to scroll through long queries. Press any other key to close.

## PostgreSQL truncation

PostgreSQL truncates `pg_stat_activity.query` at `track_activity_query_size` bytes (default: 1024). With the default, long queries appear cut off in the overlay. Raise the limit to get the most out of scrolling:

```sql
ALTER SYSTEM SET track_activity_query_size = 65536;
-- Requires a server restart to take effect
```

See [PostgreSQL setup](../../postgres-setup) for details.
