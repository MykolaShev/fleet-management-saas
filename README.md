# Fleet Management SaaS

A multi-tenant B2B platform for managing vehicle fleets and deliveries — built as a
web technologies coursework project, with a focus on production-style backend
practices (authentication, real-time updates, caching, an AI-assisted logistics
agent, and cloud deployment).

> **Status:** 🚧 Early development — project setup phase.
> This README is a living document and will be updated as features land.

## What this project does

Fleet Management SaaS lets a company (tenant) manage its own fleet of vehicles,
drivers/managers, and delivery orders in isolation from other tenants. Core ideas:

- **Multi-tenancy** — each company's data is isolated at the application/DB level.
- **Live tracking** — vehicle positions and delivery status are pushed to clients
  in real time.
- **AI logistics assistant** — a chat-style agent (with tool access via MCP) that
  can answer questions about routes, ETAs, and delivery status.
- **Delivery proof** — drivers can upload a photo confirmation for completed
  deliveries.

The business domain is intentionally kept simple — the focus of this project is on
the *technical* building blocks around it (see [Tech Stack](#tech-stack) and
[Roadmap](docs/ROADMAP.md)).

## Tech Stack

| Layer          | Technology                                              |
|----------------|----------------------------------------------------------|
| Backend        | Go, [Gin](https://github.com/gin-gonic/gin)              |
| Database       | PostgreSQL (+ PostGIS for geodata, if needed)             |
| Cache          | Redis                                                     |
| Auth           | OAuth2 (Microsoft identity platform), JWT                |
| Frontend       | React (SPA)                                               |
| Async/Queue    | RabbitMQ                                                  |
| Serverless     | AWS Lambda (Go runtime) — photo processing                |
| Storage/CDN    | Blob storage + CDN for delivery photos                     |
| Notifications  | Telegram Bot API                                          |
| Cloud          | Azure for Students / AWS Free Tier                         |
| CI/CD          | GitHub Actions                                             |

## Project Structure

```
.
├── cmd/api/                 # application entrypoint (main.go)
├── internal/
│   ├── config/               # configuration loading (env, files)
│   ├── domain/                # core entities & domain types (Tenant, User, Vehicle, Delivery, ...)
│   ├── handler/                # HTTP handlers (Gin) — request/response layer
│   ├── service/                 # business logic layer
│   ├── repository/               # data access layer (Postgres, Redis)
│   ├── middleware/                # auth, tenant isolation, logging, etc.
│   └── platform/                  # infrastructure clients (db connection, redis client, ...)
├── migrations/               # SQL schema migrations
├── deployments/
│   └── docker/                # Dockerfile, docker-compose for local dev
├── test/
│   ├── integration/            # integration tests (testcontainers, etc.)
│   └── e2e/                      # end-to-end tests (Playwright)
├── web/                       # React SPA frontend
├── docs/                      # architecture notes, roadmap, testing strategy
└── .github/workflows/          # CI pipelines
```

This layout follows a Clean Architecture-inspired separation
(`handler → service → repository`), consistent with the companion
[Expense Tracker API](https://github.com/MykolaShev/Expense-tracker-API-written-in-Go)
project.

## Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- PostgreSQL client (optional, for manual DB access)
- Node.js 18+ (later, for the frontend)

### Local setup

```bash
# 1. Clone the repo
git clone https://github.com/<your-username>/fleet-management-saas.git
cd fleet-management-saas

# 2. Copy environment variables
cp .env.example .env

# 3. Start local infrastructure (Postgres, Redis)
docker compose -f deployments/docker/docker-compose.yml up -d

# 4. Run the API
go run ./cmd/api
```

### Running tests

```bash
go test ./...
```

See [`docs/TESTING.md`](docs/TESTING.md) for the full testing strategy — including
how non-code-centric stages (infrastructure, UI, AI-agent behavior, async jobs)
are verified.

## Documentation

- [`docs/ROADMAP.md`](docs/ROADMAP.md) — implementation roadmap and current progress
- [`docs/TESTING.md`](docs/TESTING.md) — testing strategy per stage type
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — architecture decisions *(added as the project grows)*

## License

MIT — see [`LICENSE`](LICENSE).
