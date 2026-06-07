#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Building pgincident..."
go build -o pgincident ./cmd/pgincident

echo "==> Starting Postgres..."
docker compose up -d postgres
sleep 3

echo "==> Starting load generator..."
docker compose exec -T postgres psql -U postgres < dev/loadgen_setup.sql
go run ./cmd/pgincident-loadgen &
LOADGEN_PID=$!
trap 'kill $LOADGEN_PID 2>/dev/null || true' EXIT

echo "==> Waiting 5s for load to build up..."
sleep 5

echo "==> Recording demo..."
vhs demo.tape

echo "==> Done! -> demo.gif"
