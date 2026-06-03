# Usage

## Installation

See the [Installation section in the README](../README.md#installation) for Homebrew, binary download, and build-from-source options.

## Configuration

pgincident looks for a config file in this order:

1. Path given by `--config PATH`
2. `.pgincident.toml` in the current directory
3. `~/.pgincident.toml`

Run `--init` in your project directory to generate a config file with defaults:

```bash
pgincident --init
# Created /your/project/.pgincident.toml
```

Then edit the generated file and set your DSN. A minimal single-connection config:

```toml
[connections.default]
dsn = "postgres://user:password@localhost:5432/mydb"

[thresholds]
long_running        = "5s"
idle_in_transaction = "30s"
```

You can define multiple connections and switch between them at runtime:

```toml
[connections.primary]
dsn = "postgres://user:password@primary:5432/mydb"

[connections.replica]
dsn = "postgres://user:password@replica:5432/mydb"

[thresholds]
long_running        = "5s"
idle_in_transaction = "30s"
```

- **`connections.<name>.dsn`** — PostgreSQL connection string for the named preset. Supports any libpq-compatible DSN. At least one `[connections.*]` section is required.
- **`thresholds.long_running`** — queries exceeding this duration appear in the Long-running section (default: `5s`).
- **`thresholds.idle_in_transaction`** — sessions exceeding this duration appear in the Idle in transaction section (default: `30s`).

pgincident connects to the first connection defined in the file on startup. Omitting `[thresholds]` or individual threshold keys keeps the defaults.

To use a different config file, pass `--config`:

```bash
pgincident --config /path/to/other.toml
```

## Starting pgincident

Once `.pgincident.toml` is in place, run:

```bash
pgincident
```

The tool reads the config, connects to the database, and opens the TUI.

## Screens

pgincident has three screens. You start on the Overview screen.

### Overview screen

A global health summary. Shows key metrics with status badges:

| Metric | Status thresholds |
|---|---|
| Connections | WARN ≥ 80%, CRIT ≥ 90% |
| TPS | always OK (informational) |
| Cache hit | WARN < 99%, CRIT < 95% |
| Checkpoints | WARN ≥ 10 req/interval, CRIT ≥ 20 |
| Replication lag | WARN ≥ 5s, CRIT ≥ 30s (shown only when standbys exist) |
| Autovacuum | WARN ≥ 3 workers, CRIT ≥ 5 |

If a metric shows **WARN** or **CRIT**, press `o` to switch to the Dashboard and investigate.

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

### Dashboard screen

Per-category incident view with three sections. Each section auto-refreshes at the configured interval.

- **Long-running queries** — active queries exceeding the threshold (default 5 s)
- **Locks** — blocked/blocking session pairs
- **Idle in transaction** — sessions holding an open transaction beyond the threshold (default 30 s)

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

### Query detail overlay

Press `Enter` on a row in the Long-running queries section to open the full SQL. The query is formatted with clause breaks and keyword highlighting. Press any key to close.

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

## Connection switching

When multiple connections are defined in the config, press `c` on any screen to open the connection selector overlay.

```
        Select Connection

      ▶ primary  (current)
        replica

        [↑↓/jk] move  [Enter] connect  [Esc/c/q] cancel
```

Navigate with `↑`/`↓` (or `j`/`k`), then press `Enter` to switch. The title bar updates immediately to show the new connection name and PG version:

```
replica  10.0.1.42:5433  PG 17.10                             interval: 5.0s
```

**Only one database is connected at any time.** Switching closes the existing connection before opening the new one, so there is no background polling against inactive databases.

## Key bindings

### Overview screen

| Key | Action |
|---|---|
| `o` | Switch to Dashboard |
| `+` / `-` | Increase / decrease refresh interval |
| `c` | Open connection selector (only shown when multiple connections are defined) |
| `?` | Help overlay |
| `q` / `Ctrl-C` | Quit |

### Dashboard screen

| Key | Action |
|---|---|
| `o` | Switch to Overview |
| `Tab` | Move to next section |
| `Shift-Tab` | Move to previous section |
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Enter` | Open query detail overlay (Long-running queries only) |
| `+` / `-` | Increase / decrease refresh interval |
| `c` | Open connection selector (only shown when multiple connections are defined) |
| `?` | Help overlay |
| `q` / `Ctrl-C` | Quit |

### Connection selector overlay

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Enter` | Connect to selected connection |
| `Esc` / `c` / `q` | Cancel and close |

### Query detail overlay

| Key | Action |
|---|---|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| any other key | Close overlay |
