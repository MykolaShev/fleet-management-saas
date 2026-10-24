# Implementation Roadmap

This document tracks planned work items and their status. It is updated as the
project progresses.

Legend: ⬜ not started · 🟨 in progress · ✅ done

## Core + Infrastructure

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| Pet project | Standalone API, documented & polished | ✅ |
| REST API CRUD | Core CRUD endpoints (Vehicle, User, Delivery) | ✅ |
| Pagination | List endpoints support pagination       | ✅ |
| Docker | Containerized app + local dev stack         | ✅ |
| CI/CD | GitHub Actions pipeline (build, test, Docker smoke-test, Postman) | ✅ (actual *deploy* step still pending — added once C2's provider is picked) |
| Cloud deploy | Deployed to a free-tier cloud provider  | ⬜ (paused — provider decision pending, no Azure for Students available; researching alternatives) |
| API tests | Automated API test suite (Postman/Newman) | ✅ |
| Caching | Redis-backed caching layer                  | ⬜ |

## Auth / Multi-tenancy

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| OAuth2 | Login via an existing provider — switched from Microsoft to **Google** (course allows Microsoft/Google/Facebook; Microsoft Entra ID's organization/tenant setup was impractical without an existing org) | ✅ |
| Tenant isolation | Data scoped per tenant — enforced in every repository query (`WHERE tenant_id = ?`), no dedicated file, proven by the authorization test suite | ✅ |
| Auth tests | Automated auth/authorization tests (missing/garbage/expired/wrong-secret token, cross-tenant read/delete/list/assign) | ✅ |
| Feature flags | Per-tenant feature toggles           | ✅ |

## AI Agent

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| AI logistics agent | Chat agent with SSE streaming, tool-calling decide/act/answer flow, raw-SSE visibility into tool calls (no frontend yet — demoed via Postman, see §0's framing of "to the frontend") | ✅ (`go build`/`go test` green, demoed; merge of `stage/C21` into `main` still pending) |
| MCP server | 3 read tools + 1 write (C22's own minimum): `get_vehicle_location`, `get_delivery_eta` (honest stub — no live GPS yet), `list_drivers`, `update_delivery_status` | ✅ (same test/merge status as above) |

## Map + Real-time

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| Vehicle map | Leaflet/OSM demo page (`/map`, not F5) — live markers, status filter, "many points" not one pin | ✅ |
| WebSockets (C10) | WS, long poll, adaptive poll all serving the same simulated live positions, `cmd/wsbench` for the comparative analysis | ✅ (position simulator moves only non-`idle` vehicles — PATCH a vehicle's status first when demoing) |
| GraphQL | GraphQL layer over live-tracking data     | ⬜ |

## Frontend

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| SPA | React single-page application               | ⬜ |
| Autocomplete | SQL-backed search autocomplete      | ⬜ |
| Charts | Data visualizations (Chart.js)            | ⬜ |
| MSAL.js + Microsoft Graph | Microsoft-specific — needs re-scoping since auth is now Google-based. Revisit: either swap for Google Identity Services JS, or treat as a separately justified exception | ⬜ (blocked on a decision) |

## Serverless + Async

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| Lambda | AWS Lambda (Go) for photo processing      | ⬜ |
| Blob/CDN | Delivery photo storage + CDN            | ⬜ |
| Telegram notifications | Delivery event notifications | ⬜ |
| Message queue | RabbitMQ-based delivery event queue  | ⬜ |

## QA / Analytics

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| E2E tests | Playwright tests for the key user flow | ⬜ |
| Analytics | Basic usage analytics integration        | ⬜ |

## Backlog (added if time allows)

- CRM integration
- File upload API (low-level)
- Markdown editor with drag-and-drop

## Explicitly out of scope

- Server-side rendered views / SEO-focused pages
- Elasticsearch-based search
- Shared real-time canvas dashboard
- Canvas/2D game
- Web3/blockchain features