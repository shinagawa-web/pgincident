.PHONY: build run dev-up dev-down dev-seed

build:
	go build ./...

run:
	go run ./cmd/pgincident

dev-up:
	docker compose up -d
	@echo "waiting for postgres..."
	@until docker compose exec postgres pg_isready -q; do sleep 1; done
	@echo "ready. DSN: postgres://pgincident_dev:pgincident_dev@localhost:5432/postgres"

dev-down:
	docker compose down

dev-seed:
	@echo "open 4 terminals and run the commands in dev/seed.sql"
	@cat dev/seed.sql
