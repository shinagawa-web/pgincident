---
title: Dashboard screen
weight: 20
---

# Dashboard screen

The Dashboard shows three incident categories, each auto-refreshing at the configured interval.

```
primary  10.0.1.42:5432  PG 16.1                              interval: 5.0s
Connections: 142/200 (71%)   TPS: 2340   Cache hit: 99.2%
──────────────────────────────────────────────────────────────────────────
▶ Long-running queries (> 5s)                               [1 active]
  PID     USER      DURATION     STATE    QUERY
  12345   app_user  00:02:14.32  active   SELECT u.* FROM users u JOIN…
──────────────────────────────────────────────────────────────────────────
  Locks (waiting)                                           [0 waiting]
──────────────────────────────────────────────────────────────────────────
  Idle in transaction (> 30s)                               [0 idle]
──────────────────────────────────────────────────────────────────────────
[q]uit  [Tab]section  [↑↓/jk]cursor  [o]overview  [Enter]detail  [+/-]interval  [?]help
```

## Sections

**Long-running queries** — active queries exceeding the `long_running` threshold (default 5 s). Press `Enter` on a row to open the [Query detail overlay](../query-detail).

**Locks** — blocked/blocking session pairs. Shows the blocked PID, the blocking PID, wait time, relation, and lock mode.

**Idle in transaction** — sessions holding an open transaction beyond the `idle_in_transaction` threshold (default 30 s). These hold locks and block autovacuum.

## Connection switching

When multiple connections are defined in the config, press `c` to open the connection selector overlay:

```
        Select Connection

      ▶ primary  (current)
        replica

        [↑↓/jk] move  [Enter] connect  [Esc/c/q] cancel
```

Navigate with `↑`/`↓` (or `j`/`k`), then press `Enter` to switch. The title bar updates immediately to reflect the new connection.
