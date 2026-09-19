.PHONY: up down logs test lint smoke migrate tidy

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api

test:
	go test ./...

lint:
	gofmt -l .
	golangci-lint run ./...

smoke:
	pwsh -File scripts/smoke.ps1

migrate:
	migrate -path migrations -database "$(DATABASE_URL)" up

tidy:
	go mod tidy
