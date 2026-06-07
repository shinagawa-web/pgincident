---
title: Troubleshooting
weight: 60
---

# Troubleshooting

## Connection refused / wrong DSN

```
failed to connect: dial tcp ...: connect: connection refused
```

Check that the DSN in `.pgincident.toml` points to the correct host and port, and that the database is reachable from your machine.

## Permission denied (`pg_monitor`)

```
ERROR: permission denied for view pg_stat_activity
hint: GRANT pg_monitor TO your_user;
```

pgincident requires the `pg_monitor` role. Grant it and reconnect:

```sql
GRANT pg_monitor TO your_user;
```

See [PostgreSQL setup](postgres-setup) for details.

## Terminal too small

pgincident requires a minimum terminal size of **80 columns × 24 rows**. Below this, a warning screen is shown instead of the dashboard. Resize your terminal window and the TUI will recover automatically.

## Queries appear truncated in the detail overlay

PostgreSQL's `track_activity_query_size` setting (default: 1024 bytes) truncates long queries before pgincident ever sees them. Raise the limit — see [PostgreSQL setup](postgres-setup#raising-track_activity_query_size).

## TPS shows as `—` after a server restart

TPS is calculated as the delta of `xact_commit + xact_rollback` between polls. When the server restarts or `pg_stat_reset()` is called, the counter goes backward. pgincident skips the TPS value for that interval and shows `—` instead of a negative number. It recovers on the next poll.
