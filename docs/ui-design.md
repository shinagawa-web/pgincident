# UI Design

> **Forward-looking design document.** This describes the target navigation model across multiple versions. v0.1 ships a single dashboard screen (Level 2 entry point). Level 1 is planned for v0.1.3; Level 3 for v0.3.

## Three-level navigation

### Level 1 — Overview (entry point)

"Is something wrong?" — key indicators with status colors (normal / warning / critical).

| Metric | Source |
|---|---|
| Connections / max % | `pg_stat_activity`, `pg_settings` |
| TPS | `pg_stat_database` |
| Cache hit ratio | `pg_stat_database` |
| Long-running query count | `pg_stat_activity` |
| Lock wait count | `pg_locks` |
| Idle in transaction count | `pg_stat_activity` |
| Checkpoint frequency | `pg_stat_bgwriter` |
| WAL generation rate | `pg_stat_wal` (PG14+) |
| Replication lag | `pg_stat_replication` |
| Autovacuum active count | `pg_stat_progress_vacuum` |

### Level 2 — Category view (drill down when something is red)

| Category | Content |
|---|---|
| Activity | Long-running queries, idle in transaction |
| Locks | Blocked/blocking pairs |
| I/O | bgwriter, checkpoint, buffer stats |
| Statements | pg_stat_statements slow query history |
| Tables | High seq scan tables, dead tuple ratio |
| Vacuum | Autovacuum progress, bloat |
| Replication | Lag, state |
| Connections | Breakdown by state / user / application; PgBouncer stats |

### Level 3 — Process view (individual investigation)

Enter from a selected row in the category view. Shows full SQL, wait events, lock chain. cancel (`c`) / kill (`K`) available with `pg_signal_backend`.
