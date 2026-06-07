---
title: Overview screen
weight: 10
---

# Overview screen

The Overview screen is the entry point. It shows global health indicators with status colors so you can tell at a glance whether the database is healthy.

```
primary  10.0.1.42:5432  PG 16.1                              interval: 5.0s
──────────────────────────────────────────────────────────────────────────
  DB Health Overview
──────────────────────────────────────────────────────────────────────────

  Metric                Value                 Status
  ──────────────────────────────────────────────────
  Connections           142 / 200 (71%)       OK
  TPS                   2340                  OK
  Cache hit             99.2%                 OK
  Checkpoints           req: 0                OK
  Autovacuum            0 workers             OK

──────────────────────────────────────────────────────────────────────────
[o]dashboard  [q]uit  [+/-]interval  [?]help
```

## Status thresholds

| Metric | WARN | CRIT |
|---|---|---|
| Connections | ≥ 80% | ≥ 90% |
| TPS | — (informational) | — |
| Cache hit | < 99% | < 95% |
| Checkpoints | ≥ 10 req/interval | ≥ 20 req/interval |
| Replication lag | ≥ 5 s | ≥ 30 s (shown only when standbys exist) |
| Autovacuum | ≥ 3 workers | ≥ 5 workers |

If any metric shows **WARN** or **CRIT**, press `o` to switch to the [Dashboard](../dashboard-screen) and investigate.
