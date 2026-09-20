# Testing Strategy

Every implemented stage must have accompanying evidence that it works — not
every stage fits a plain unit-test model, so this document defines what counts
as acceptable proof for each *kind* of stage. As new stages are implemented,
add a row to the table below with a link to the actual test/proof.

## Categories

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
| _(filled in as stages are completed)_ | | | |

This table is the source of truth for "this stage is done and provably so" —
update it as part of finishing each stage, not as a separate cleanup pass.
