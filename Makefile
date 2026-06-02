.PHONY: build run test test-integration lint tidy install-hooks dev-up dev-down dev-seed dev-load dev-ssl-up dev-ssl-down

VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
LDFLAGS := -X github.com/shinagawa-web/pgincident/internal/version.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o pgincident ./cmd/pgincident/

run:
	go run ./cmd/pgincident --config .pgincident.dev.toml

test:
	env -u DATABASE_URL go test -race -coverprofile=coverage.out -covermode=atomic ./internal/...

test-integration:
	go test -race -v -timeout 2m -run Integration ./internal/core/...

lint:
	go vet ./...

tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

install-hooks:
	cp scripts/pre-push .git/hooks/pre-push
	chmod +x .git/hooks/pre-push
	@echo "pre-push hook installed"

dev-up:
	docker compose up -d
	@echo "waiting for postgres and postgres-b..."
	@until docker compose exec postgres pg_isready -q; do sleep 1; done
	@until docker compose exec postgres-b pg_isready -q; do sleep 1; done
	docker compose exec -T postgres psql -U postgres < dev/loadgen_setup.sql
	@printf '[connections.primary]\ndsn = "postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"\n\n[connections.replica]\ndsn = "postgres://pgincident_dev:pgincident_dev@localhost:5433/postgres"\n\n[thresholds]\nlong_running        = "5s"\nidle_in_transaction = "30s"\n' > .pgincident.dev.toml
	@echo "ready."
	@echo "  primary (port 5432): postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"
	@echo "  replica (port 5433): postgres://pgincident_dev:pgincident_dev@localhost:5433/postgres"
	@echo "  config written to .pgincident.dev.toml  (make run to start TUI)"

dev-down:
	docker compose down

dev-ssl-up:
	docker compose up -d postgres-ssl
	@echo "waiting for postgres-ssl..."
	@until docker compose exec postgres-ssl pg_isready -q; do sleep 1; done
	docker compose exec -T postgres-ssl psql -U postgres < dev/loadgen_setup.sql
	@grep -q '^\[connections\.rds\]' .pgincident.dev.toml 2>/dev/null || \
		printf '\n[connections.rds]\ndsn = "postgres://pgincident_dev:pgincident_dev@localhost:5434/postgres?sslmode=require"\n' >> .pgincident.dev.toml
	@echo "ready."
	@echo "  rds (port 5434): postgres://pgincident_dev:pgincident_dev@localhost:5434/postgres?sslmode=require"
	@echo "  [connections.rds] added to .pgincident.dev.toml"

dev-ssl-down:
	docker compose stop postgres-ssl
	docker compose rm -f postgres-ssl

dev-seed:
	@echo "open 4 terminals and run the commands in dev/seed.sql"
	@cat dev/seed.sql

# dev-load: spin up a realistic, sustained workload against the dev Postgres.
# All four subsystems (tps/slow/locks/idle) are enabled by default.
# Pass LOADGEN_FLAGS to toggle subsystems, e.g.:
#   make dev-load LOADGEN_FLAGS="--no-locks --no-idle"
dev-load:
	@pkill -f pgincident-loadgen 2>/dev/null || true
	docker compose exec -T postgres psql -U postgres < dev/loadgen_setup.sql
	go run ./cmd/pgincident-loadgen $(LOADGEN_FLAGS)
