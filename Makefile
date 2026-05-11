.PHONY: build run test test-integration lint tidy install-hooks dev-up dev-down dev-seed dev-load

build:
	go build ./...

run:
	go run ./cmd/pgincident

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
	@echo "waiting for postgres..."
	@until docker compose exec postgres pg_isready -q; do sleep 1; done
	docker compose exec -T postgres psql -U postgres < dev/loadgen_setup.sql
	@echo "ready. DSN: postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"

dev-down:
	docker compose down

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
