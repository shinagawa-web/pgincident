---
title: Contributing
weight: 70
---

# Contributing

## Prerequisites

- Go 1.25+
- Docker (for local Postgres)

## Dev environment

```sh
make dev-up   # start Postgres 16 with pgincident_dev / pg_monitor
make run      # build and launch the TUI
make dev-down # stop and remove the container
```

## Make targets

| Target | Description |
|---|---|
| `make test` | Run all unit and e2e tests with race detector |
| `make test-integration` | Run integration tests (requires `DATABASE_URL`) |
| `make lint` | Run `go vet` |
| `make tidy` | Run `go mod tidy` and verify no diff |
| `make install-hooks` | Install the pre-push hook |
| `make dev-load` | Start the workload simulator against the dev DB |

## Pre-push hook

Install once after cloning:

```sh
make install-hooks
```

The hook runs on every `git push` and checks: `go mod tidy`, `go build`, `go vet`, unit + e2e tests, and integration tests. Integration tests require `DATABASE_URL` — start the dev container and export the DSN before pushing:

```sh
make dev-up
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
git push
```

## Workload simulator

`make dev-load` runs a simulator that creates long-running queries, lock contention, and idle-in-transaction sessions against the dev DB — the fastest way to evaluate the full dashboard under realistic conditions.

```sh
# Terminal 1
make dev-load

# Terminal 2
make run
```

Within ~10 seconds all dashboard sections show non-empty, time-varying data. The Idle in transaction section takes ~30 s to populate (that is the detection threshold).

## Further reading

- [Architecture](architecture) — three-layer separation (core / tui / version)
- [Incident scenarios](incident-scenarios) — manual reproduction steps for each dashboard section
