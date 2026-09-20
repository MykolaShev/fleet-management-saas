# Implementation Roadmap

This document tracks planned work items and their status. It is updated as the
project progresses.

Legend: ⬜ not started · 🟨 in progress · ✅ done

## Core + Infrastructure

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| Pet project | Standalone API, documented & polished | ⬜ |
| REST API CRUD | Core CRUD endpoints                  | ⬜ |
| Pagination | List endpoints support pagination       | ⬜ |
| Docker | Containerized app + local dev stack         | ⬜ |
| CI/CD | GitHub Actions pipeline (lint/test/build)    | ⬜ |
| Cloud deploy | Deployed to a free-tier cloud provider  | ⬜ |
| API tests | Automated API test suite (Postman/Newman) | ⬜ |
| Caching | Redis-backed caching layer                  | ⬜ |

## Auth / Multi-tenancy

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| OAuth2 | Login via Microsoft identity platform      | ⬜ |
| Tenant isolation | Data scoped per tenant             | ⬜ |
| Auth tests | Automated auth/authorization tests    | ⬜ |
| Feature flags | Per-tenant feature toggles           | ⬜ |

## AI Agent

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| AI logistics agent | Chat agent with SSE streaming | ⬜ |
| MCP server | Tools: coordinates, ETA, status update  | ⬜ |

## Map + Real-time

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| Vehicle map | Map view with filtering (Leaflet/OSM)| ⬜ |
| WebSockets | Live position tracking                 | ⬜ |
| GraphQL | GraphQL layer over live-tracking data     | ⬜ |

## Frontend

| Item | Description                          | Status |
|------|---------------------------------------|--------|
| SPA | React single-page application               | ⬜ |
| Autocomplete | SQL-backed search autocomplete      | ⬜ |
| Charts | Data visualizations (Chart.js)            | ⬜ |
| MSAL.js | Microsoft Graph integration on frontend  | ⬜ |

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
