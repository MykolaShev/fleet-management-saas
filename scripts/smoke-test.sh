#!/usr/bin/env bash
# Smoke test for the Docker stack (C4 evidence).
#
# Builds the api image, brings up postgres+redis+api via docker compose,
# waits for the /health endpoint to respond, mints a dev-only session
# token (requires APP_ENV=local, which docker-compose.yml sets for the api
# service), exercises one real CRUD call end-to-end through that token,
# then tears everything down.
#
# Usage: ./scripts/smoke-test.sh
# Exit code is non-zero on any failure — this is the same script C3's CI
# job calls.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/deployments/docker/docker-compose.yml"
BASE_URL="http://localhost:8080"

cleanup() {
  echo "==> Tearing down..."
  docker compose -f "$COMPOSE_FILE" --profile full down -v
}
trap cleanup EXIT

echo "==> Building and starting the full stack (postgres + redis + api)..."
docker compose -f "$COMPOSE_FILE" --profile full up -d --build

echo "==> Waiting for /health..."
attempts=30
until curl -sf "$BASE_URL/health" >/dev/null 2>&1; do
  attempts=$((attempts - 1))
  if [ "$attempts" -le 0 ]; then
    echo "FAIL: API did not become healthy in time"
    docker compose -f "$COMPOSE_FILE" --profile full logs api
    exit 1
  fi
  sleep 2
done
echo "OK: /health responded"

echo "==> Minting a dev session token (C16 dev-token escape hatch)..."
token_status=$(curl -s -o /tmp/smoke-token.json -w "%{http_code}" \
  -X POST "$BASE_URL/api/v1/auth/dev-token" \
  -H "Content-Type: application/json" \
  -d '{"role":"manager"}')

if [ "$token_status" != "200" ]; then
  echo "FAIL: expected 200 from POST /api/v1/auth/dev-token, got $token_status"
  cat /tmp/smoke-token.json
  exit 1
fi

token=$(grep -o '"token":"[^"]*"' /tmp/smoke-token.json | head -1 | cut -d'"' -f4)
if [ -z "$token" ]; then
  echo "FAIL: could not extract token from dev-token response"
  cat /tmp/smoke-token.json
  exit 1
fi
echo "OK: got a session token"

echo "==> Exercising Vehicle CRUD against the real containerized stack..."
create_status=$(curl -s -o /tmp/smoke-create.json -w "%{http_code}" \
  -X POST "$BASE_URL/api/v1/vehicles" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"plate_number":"SMOKE1","model":"Smoke Test Van"}')

if [ "$create_status" != "201" ]; then
  echo "FAIL: expected 201 from POST /api/v1/vehicles, got $create_status"
  cat /tmp/smoke-create.json
  exit 1
fi
echo "OK: created vehicle -> $(cat /tmp/smoke-create.json)"

vehicle_id=$(grep -o '"id":"[^"]*"' /tmp/smoke-create.json | head -1 | cut -d'"' -f4)

get_status=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE_URL/api/v1/vehicles/$vehicle_id" \
  -H "Authorization: Bearer $token")

if [ "$get_status" != "200" ]; then
  echo "FAIL: expected 200 from GET /api/v1/vehicles/:id, got $get_status"
  exit 1
fi
echo "OK: fetched the vehicle back"

echo "==> Smoke test passed."