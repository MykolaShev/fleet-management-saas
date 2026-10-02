.PHONY: run build test lint up up-full down tidy smoke-test postman-test

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

up-full:
	docker compose -f deployments/docker/docker-compose.yml --profile full up -d --build

down:
	docker compose -f deployments/docker/docker-compose.yml --profile full down

tidy:
	go mod tidy

smoke-test:
	./scripts/smoke-test.sh

postman-test:
	npx --yes newman run test/postman/fleet-management.postman_collection.json \
		-e test/postman/fleet-management-local.postman_environment.json