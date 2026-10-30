.PHONY: run build test lint up up-full down tidy smoke-test postman-test clean-testcache

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

# --env-file is required here (not for 'up'/'down' above): without it,
# docker compose looks for .env next to the compose file
# (deployments/docker/), not the repo root where it actually lives, and
# LLM_API_KEY silently never reaches the api container.
up-full:
	docker compose -f deployments/docker/docker-compose.yml --env-file .env --profile full up -d --build

down:
	docker compose -f deployments/docker/docker-compose.yml --env-file .env --profile full down

tidy:
	go mod tidy

smoke-test:
	bash scripts/smoke-test.sh

postman-test:
	npx --yes newman run test/postman/fleet-management.postman_collection.json \
		-e test/postman/fleet-management-local.postman_environment.json

clean-testcache:
	go clean -testcache