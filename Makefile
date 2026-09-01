MIGRATIONS_DIR := api/internal/store/migrations
DATABASE_URL   ?= postgres://asr:asr@localhost:5432/asr?sslmode=disable

.PHONY: up down logs migrate-up migrate-down sqlc test lint

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

sqlc:
	cd api && sqlc generate

test:
	cd api && go test -race -cover ./...
	cd worker && uv run pytest
	cd web && npm test --if-present

lint:
	cd api && golangci-lint run
	cd worker && uv run ruff check .
	cd web && npm run lint
