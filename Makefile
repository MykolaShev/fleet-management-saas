.PHONY: run build test lint up down tidy

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -v

lint:
	golangci-lint run ./...

up:
	docker compose -f deployments/docker/docker-compose.yml up -d

down:
	docker compose -f deployments/docker/docker-compose.yml down

tidy:
	go mod tidy
