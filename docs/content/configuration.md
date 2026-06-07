---
title: Configuration
weight: 20
---

# Configuration

pgincident looks for a config file in this order:

1. Path given by `--config PATH`
2. `.pgincident.toml` in the current directory
3. `~/.pgincident.toml`

Run `--init` to generate a config file with defaults:

```sh
pgincident --init
# Created /your/project/.pgincident.toml
```

## Minimal config

```toml
[connections.default]
dsn = "postgres://user:password@localhost:5432/mydb"

[thresholds]
long_running        = "5s"
idle_in_transaction = "30s"
```

## Multiple connections

Define multiple connection presets and switch between them at runtime with `c`:

```toml
[connections.primary]
dsn = "postgres://user:password@primary:5432/mydb"

[connections.replica]
dsn = "postgres://user:password@replica:5432/mydb"

[thresholds]
long_running        = "5s"
idle_in_transaction = "30s"
```

pgincident connects to the first connection defined in the file on startup. Only one database is active at any time — switching closes the existing connection before opening the new one.

## Reference

| Key | Type | Default | Description |
|---|---|---|---|
| `connections.<name>.dsn` | string | — | libpq-compatible connection string. At least one `[connections.*]` section is required. |
| `thresholds.long_running` | duration | `5s` | Queries exceeding this duration appear in the Long-running section. |
| `thresholds.idle_in_transaction` | duration | `30s` | Sessions exceeding this duration appear in the Idle in transaction section. |

Omitting `[thresholds]` or individual threshold keys keeps the defaults.

## Using a custom config path

```sh
pgincident --config /path/to/other.toml
```
