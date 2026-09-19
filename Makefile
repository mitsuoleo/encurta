.PHONY: up down test lint migrate tidy

up:
	docker compose up --build

down:
	docker compose down

test:
	go test ./...

lint:
	gofmt -l .
	golangci-lint run ./...

migrate:
	migrate -path migrations -database "$(DATABASE_URL)" up

tidy:
	go mod tidy
