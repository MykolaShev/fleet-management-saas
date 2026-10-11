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

### 6. Auth / authorization (OAuth2 + multi-tenancy)
The OAuth2 handshake itself (redirect to the provider, human login, consent
screen) is not automatable without a real test account and browser
automation — it's demonstrated manually / on video, not in CI.

**Proof:**
- Token issuance/validation logic (our own JWT): fully unit-testable —
  issue/parse roundtrip, expiry rejection, tampered-signature rejection.
- Claims → tenant/user mapping business logic: unit-tested with fake claims,
  no real provider involved.
- Authorization (does a valid/invalid/cross-tenant token get the right
  response): end-to-end test against the real router + real middleware,
  in-memory DB — this is where tenant isolation is actually proven, since
  there's no dedicated "isolation" code to unit-test in isolation.

## Stage → Evidence Log

| Stage | Category | Evidence type | Location |
|-------|----------|----------------|----------|
| C5 (Vehicle) / B1 | code-centric | unit tests (service, fake repo) + integration tests (repository, in-memory SQLite) | `internal/service/vehicle_service_test.go`, `internal/repository/vehicle_repository_test.go` |
| C5 (User) | code-centric | unit tests (service, fake repo) + integration tests (repository, in-memory SQLite, incl. tenant-scoped email uniqueness) | `internal/service/user_service_test.go`, `internal/repository/user_repository_test.go` |
| C5 (Delivery) / B1 | code-centric | unit tests (service, fake repo + fake vehicle/user lookups, incl. cross-tenant assignment rejection) + integration tests (repository, in-memory SQLite, incl. assign/unassign NULL persistence) | `internal/service/delivery_service_test.go`, `internal/repository/delivery_repository_test.go` |
| C4 | infrastructure | smoke-test script: `docker compose up --build`, poll `/health`, exercise real Vehicle CRUD against the containerized stack, then tear down | `scripts/smoke-test.sh` (run via `make smoke-test`) |
| QA1 | code-centric (black-box) | Postman/Newman collection: full CRUD + pagination on all 3 entities, plus business-rule checks (tenant-scoped email uniqueness, cross-tenant vehicle/driver assignment rejection) | `test/postman/fleet-management.postman_collection.json` (run via `make postman-test`) |
| B6 | code-centric | unit tests (service, fake repo: catalog merge, unknown-key rejection, default-false behavior) + integration tests (repository, in-memory SQLite: upsert not duplicate, tenant scoping) + Postman folder 4 | `internal/service/feature_flag_service_test.go`, `internal/repository/feature_flag_repository_test.go` |
| C3 | infrastructure | the pipeline itself passing on every push/PR (green GitHub Actions badge in README) — runs `go test`, the C4 smoke-test, and the QA1 Postman collection, all on GitHub's own runner | `.github/workflows/ci.yml` |
| C16 | auth (code-centric + manual) | unit tests (JWT issue/parse incl. expiry/tamper rejection; AuthService claims→tenant/user mapping with fakes, incl. new-tenant-per-new-user and returning-user reuse) — the live Google redirect itself is demo-only, confirmed manually via a real browser login, not automatable without a real test account | `internal/auth/jwt_test.go`, `internal/service/auth_service_test.go` |
| QA3 (covers B5 too) | auth (end-to-end, real HTTP) | full router + real `Authenticate` middleware against in-memory SQLite: missing/garbage/expired/wrong-secret token → 401; cross-tenant read/delete/list/assign → 404 or 422; feature-flag isolation. This is also the only evidence for B5 (tenant isolation) — there's no separate file for it, since isolation is a property of every repository's `WHERE tenant_id = ?`, not a standalone feature | `test/integration/authorization_test.go` |
| _(filled in as further stages are completed)_ | | | |