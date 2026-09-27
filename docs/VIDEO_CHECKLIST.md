# Video Demonstration Checklist

For each completed stage, this file records exactly what must be shown on
camera to prove it works — filled in right after the stage is finished (while
context is fresh), not right before submission.

Keep each entry short and concrete: what to click/run, and what result on
screen proves the stage works.

## How to fill a row

- **Stage** — course code (e.g. `C5`, `B1`, `C16`)
- **What to show** — the exact sequence of actions/screens that demonstrates
  the feature works (not just "it exists")
- **Notes** — anything to prepare beforehand (seed data, two browser windows,
  a specific test account, etc.)

## Checklist

| Stage | What to show | Notes |
|-------|--------------|-------|
| C5 (Vehicle) / B1 | `go test ./...` green in terminal; then via curl/Postman: `POST /api/v1/vehicles` (create), `GET /api/v1/vehicles?page=1&page_size=2` (show pagination metadata with 3+ vehicles), `GET /api/v1/vehicles/:id`, `PATCH` a field, `DELETE`, and one `404` on a bad id | Needs `X-Tenant-ID: <any-uuid>` header on every request (auth isn't wired up yet) |
| C5 (User) | Same CRUD flow as Vehicle on `/api/v1/users`, plus: create two users with the same email in the same tenant → `409`; create the same email again but with a different `X-Tenant-ID` → succeeds (proves tenant-scoped uniqueness) | Role must be `"manager"` or `"driver"` — show one `422` for an invalid role too |
| C5 (Delivery) / B1 | Create a delivery with no vehicle/driver → `status: pending`. Create one with a valid `vehicle_id`+`driver_id` (from the same tenant) → `status: assigned`. **Key demo:** try creating a delivery using a vehicle/driver ID that belongs to a *different* tenant → `422` (this is the cross-tenant guard). Then `PATCH` with `unassign_vehicle: true` and show `vehicle_id` becomes `null` | Create a second tenant's vehicle first (own `X-Tenant-ID`) so you have a real "foreign" ID to demonstrate the rejection with |
| C4 | Run `make smoke-test` in terminal, showing: image build, all 3 containers starting, health-check passing, and the scripted `POST`/`GET` against `/api/v1/vehicles` succeeding — then teardown | This is the single most convincing "it's really containerized and really works" demo, since it runs against the built image, not `go run` |
| QA1 | Run `make postman-test` (or open the collection in the Postman GUI and hit Run) against a live `docker compose up` stack — show the full pass summary, especially the 409 (duplicate email), 422 (cross-tenant vehicle), and 404-after-delete assertions passing | Needs a **fresh** database — the collection creates real records and isn't idempotent; re-running against old data may fail the duplicate-email/pagination-count assertions |
| _(filled in as further stages are completed)_ | | |