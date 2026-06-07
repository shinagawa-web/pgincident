---
title: PostgreSQL setup
weight: 40
---

# PostgreSQL setup

## Required privilege: `pg_monitor`

pgincident uses `pg_monitor` instead of `SUPERUSER`. This makes it work on managed databases (RDS, Cloud SQL, Aurora) where superuser access is not available.

Grant the role to your user:

```sql
GRANT pg_monitor TO your_user;
```

pgincident checks `pg_monitor` membership at startup. If the user is not a member, the tool exits with an actionable message including the grant command.

## Raising `track_activity_query_size`

PostgreSQL truncates `pg_stat_activity.query` at `track_activity_query_size` bytes. The default is **1024 bytes**, which cuts off long queries in the [query detail overlay](usage/query-detail).

Check the current value:

```sql
SHOW track_activity_query_size;
```

Raise it permanently (requires superuser + server restart):

```sql
ALTER SYSTEM SET track_activity_query_size = 65536;
SELECT pg_reload_conf(); -- not enough alone; a restart is required
```

On managed databases (RDS, Cloud SQL), set the parameter in the parameter group and reboot the instance. The local dev container (`docker-compose.yml`) already sets `track_activity_query_size=65536`.
