.PHONY: dev-web dev-api build web-build up down logs test tidy

# Build the Svelte SPA into web/dist (required before building the Go binary).
web-build:
	cd web && npm install && npm run build

# Build the Go API binary (embeds the SPA).
build: web-build
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

# Run the frontend dev server (proxies /api to :8080).
dev-web:
	cd web && npm run dev

# Run the API locally (expects a Postgres reachable via DATABASE_URL).
dev-api:
	go run ./cmd/api

# Docker: bring the whole stack up / down.
up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	go test ./...

tidy:
	go mod tidy
