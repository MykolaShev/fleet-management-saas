# Testing Strategy

Every implemented stage must have accompanying evidence that it works — not
every stage fits a plain unit-test model, so this document defines what counts
as acceptable proof for each *kind* of stage. As new stages are implemented,
add a row to the table below with a link to the actual test/proof.

## Categories

### 0. Lesson learned: SQLite ≠ Postgres on foreign keys

Our repository tests run against in-memory SQLite (fast, no Docker needed in
CI) instead of real Postgres. This caught us once already: **SQLite does not
enforce foreign keys by default**, while Postgres always does. A test that
inserted a Vehicle/User/Delivery referencing a Tenant that was never created
passed happily on SQLite and then failed in production with a foreign key
violation the first time it hit real Postgres.

Fix: `PRAGMA foreign_keys = ON` is now set explicitly in `setupTestDB`
(`internal/repository/vehicle_repository_test.go`), and every test that
inserts a row now creates its referenced Tenant/Vehicle/User first via
`createTestTenant` or a real repository call. Keep this in mind for any new
FK relationship added later: SQLite will silently let it through unless this
pragma is set.

### 1. Code-centric logic
CRUD, pagination, tenant isolation, feature flags, caching, auth logic, search.

**Proof:** standard unit/integration tests (`go test`, `testify`).

### 2. Infrastructure (Docker, CI/CD, cloud deploy)
You can't "unit test" a deployment — the proof is an automated check of system
state.

**Proof:**
- Docker: an integration test that runs `docker compose up`, waits for a
  health check, hits an endpoint, then tears down.
- CI/CD: the pipeline itself passing on every push/PR (green badge in README +
  link to a successful run).
- Cloud deploy: a `/health` endpoint, checked either manually (documented
  `curl` + timestamp) or automatically via a CI job that deploys and pings the
  live URL.

### 3. UI / visual (map, charts, SPA)
Not testing "does it look right" — testing that the underlying logic and data
flow are correct.

**Proof:**
- Map filtering: component/E2E test asserting that applying a filter
  hides/shows the correct markers.
- Charts: a test asserting the API data is correctly transformed into the
  chart's input props (not a pixel/visual comparison).
- SPA: one end-to-end smoke test covering routing and base rendering
  (may be shared with the E2E stage rather than duplicated).

### 4. AI agent (non-deterministic behavior)
Never assert on exact LLM output text.

**Proof:**
- Structural contract tests: SSE stream emits well-formed events; MCP tool
  calls return correctly-shaped JSON.
- Deterministic logic around the agent: routing, error handling, timeouts,
  fallback behavior — tested normally.
- Business logic that consumes the agent is tested with a mocked LLM response.
  A real end-to-end call against the live LLM is treated as a manual/smoke
  check, not part of the automated suite.

### 5. Async / external integrations (Lambda, queue, notifications)
Tested at the system boundary, with the external dependency mocked or
containerized.

**Proof:**
- Lambda: unit test of the handler function with a mocked event (e.g. via
  local invocation), no real deployment required in the test.
- Message queue: integration test using testcontainers — publish an event,
  assert the consumer processed it and state changed accordingly.
- Notifications (Telegram): unit test asserting the service builds the
  correct payload and calls the client, with the HTTP client mocked — real
  delivery is not exercised in automated tests.

## Stage → Evidence Log

| Stage | Category | Evidence type | Location |
|-------|----------|----------------|----------|
| C5 (Vehicle) / B1 | code-centric | unit tests (service, fake repo) + integration tests (repository, in-memory SQLite) | `internal/service/vehicle_service_test.go`, `internal/repository/vehicle_repository_test.go` |
| C5 (User) | code-centric | unit tests (service, fake repo) + integration tests (repository, in-memory SQLite, incl. tenant-scoped email uniqueness) | `internal/service/user_service_test.go`, `internal/repository/user_repository_test.go` |
| C5 (Delivery) / B1 | code-centric | unit tests (service, fake repo + fake vehicle/user lookups, incl. cross-tenant assignment rejection) + integration tests (repository, in-memory SQLite, incl. assign/unassign NULL persistence) | `internal/service/delivery_service_test.go`, `internal/repository/delivery_repository_test.go` |
| C4 | infrastructure | smoke-test script: `docker compose up --build`, poll `/health`, exercise real Vehicle CRUD against the containerized stack, then tear down | `scripts/smoke-test.sh` (run via `make smoke-test`) |
| QA1 | code-centric (black-box) | Postman/Newman collection: full CRUD + pagination on all 3 entities, plus business-rule checks (tenant-scoped email uniqueness, cross-tenant vehicle/driver assignment rejection) | `test/postman/fleet-management.postman_collection.json` (run via `make postman-test`) |
| _(filled in as further stages are completed)_ | | | |

This table is the source of truth for "this stage is done and provably so" —
update it as part of finishing each stage, not as a separate cleanup pass.