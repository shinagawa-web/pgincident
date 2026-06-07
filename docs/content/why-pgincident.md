---
title: Why pgincident?
weight: 50
---

# Why pgincident?

## Comparison

| | pgcenter | pg_activity | pgincident |
|---|---|---|---|
| Language | Go | Python | Go |
| OS | Linux only | Linux/Mac | Linux + Mac |
| Required privilege | SUPERUSER | SUPERUSER | **`pg_monitor`** |
| Managed DB (RDS, Cloud SQL) | partial | partial | **first-class** |
| Focus | comprehensive stats | top-style activity | **incident response** |
| Post-mortem export | no | no | planned |

## Three design decisions

**1. `pg_monitor` instead of SUPERUSER** — unlocks managed PostgreSQL (RDS, Cloud SQL, Aurora). Most production databases don't give you superuser. pgincident is designed for that reality.

**2. Incident-response framing** — not "show me everything", but "what's broken right now." The tool opens on the Overview screen and stays focused on the question: is there a problem, and if so, where?

**3. Overview → category → process flow** — global health first (Level 1), drill into the problem area (Level 2), then investigate individual sessions (Level 3, coming in v0.3). You shouldn't need to know which view to open; the tool guides you.

## Non-goals

- Replacing pgAdmin / DBeaver (no schema browsing, no query editor)
- Long-term metrics storage (Prometheus + Grafana already do this)
- System stats (CPU/IO/mem) — pgcenter covers this; pgincident focuses on Postgres internals
- Multi-instance dashboard (one connection at a time for now)
