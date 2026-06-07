---
title: Usage
weight: 30
---

# Usage

Once `.pgincident.toml` is in place, run:

```sh
pgincident
```

The tool reads the config, connects to the database, and opens the TUI. You start on the [Overview screen](overview-screen).

## Screens

| Screen | Purpose |
|---|---|
| [Overview](overview-screen) | Global health at a glance — spot what's wrong |
| [Dashboard](dashboard-screen) | Per-category incident view — long-running queries, locks, idle in transaction |
| [Query detail overlay](query-detail) | Full SQL for a selected query with scrolling |

## Key bindings

See the [Key bindings reference](key-bindings) for all shortcuts.
