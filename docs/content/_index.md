---
title: pgincident
type: docs
---

# pgincident

> "The first 30 seconds of a Postgres incident — in one terminal."

![pgincident demo](demo.gif)

Production Postgres is slow. pgincident collapses the usual sequence of manual `psql` queries into a live TUI: a global health overview to spot the problem, then a per-category dashboard to dig in.

## Quick install

```sh
# Homebrew (macOS)
brew tap shinagawa-web/tap
brew install pgincident

# Linux / macOS one-liner
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/pgincident/main/install.sh | sh
```

→ [Full installation guide](installation)

## Documentation

- [Installation](installation)
- [Configuration](configuration)
- [Usage](usage)
- [PostgreSQL setup](postgres-setup)
- [Why pgincident?](why-pgincident)
- [Troubleshooting](troubleshooting)
- [Contributing](contributing)
