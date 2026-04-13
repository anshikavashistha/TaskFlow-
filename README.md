# TaskFlow (backend)

## Overview

TaskFlow is a small task-management REST API built with **Go 1.22**, **PostgreSQL 16**, **Chi** for routing, **pgx** for Postgres access, **golang-migrate** (embedded SQL files) for schema changes, **JWT** (HS256) for authentication, **bcrypt** (cost **12**) for passwords, and **slog** for structured logs. There is no React frontend; use the Postman collection, curl, or the optional integration tests instead.

## Architecture decisions

- **Chi** keeps routing and middleware explicit without heavy framework magic; `net/http` compatibility is excellent for tests with `httptest`.
- **pgx** and hand-written SQL give clear control over queries (pagination, stats `GROUP BY`, visibility rules) without ORM surprises for a bounded assignment.
- **Handlers vs `internal/db`**: HTTP types and validation live in `handlers`; persistence and SQL stay in `db` for easier reading and testing.
- **Migrations embedded** via `//go:embed` so the binary stays self-contained in Docker (no bind-mount for migration files required at runtime).
- **Seeding** runs inside the API after migrations when `SEED=true`, so Postgres init scripts never run against tables that do not exist yet (migrations run first).

## Running locally

1. Install **Go 1.22+** and **PostgreSQL 16** (or use Docker only for the database).
2. `cd backend` and run `go mod tidy` (generates `go.sum` on first clone).
3. Create a database (e.g. `taskflow`) and copy `.env.example` to `.env` at the repo root. For a local Postgres, set `DB_HOST=localhost` and matching credentials.
4. Set a strong `JWT_SECRET` in `.env` (never commit `.env`).
5. From `backend`: `go run ./cmd/server`

The server applies **migrations automatically** on startup, then optional seed if `SEED=true`.

## Running migrations

Migrations run automatically when the server starts (`db.RunMigrations`). SQL lives in `backend/migrations/` with matching `.up.sql` and `.down.sql` files.

To use **golang-migrate CLI** against the same database (optional):

```bash
cd backend
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down 1
```

(`DATABASE_URL` should be a `postgres://` URL with `sslmode=disable` for local dev.)

## Docker

From the repository root (after copying `.env.example` to `.env` and setting `JWT_SECRET`):

```bash
docker compose up --build
```

- API: `http://localhost:8080` (or the host port mapped from `PORT` in `.env`).
- Postgres is internal on hostname `db`; the example `.env` uses `DB_HOST=db` for the API container.

## Test credentials

When `SEED=true` (default in `.env.example`), the API inserts (once) a demo account:

- **Email:** `test@example.com`
- **Password:** `password123`

You can also register a new user via `POST /auth/register`.

## API reference

Base URL: `http://localhost:8080` (adjust as needed).

Errors are JSON: `{"error":"message"}`. Typical status codes: **400** validation, **401** missing/invalid JWT or bad login, **403** not allowed, **404** missing resource, **500** unexpected server error.

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/register` | No | Body: `name`, `email`, `password`. **201** + `{ token, user }`. Duplicate email **400**. |
| POST | `/auth/login` | No | Body: `email`, `password`. **200** + `{ token, user }`. Wrong credentials **401**. |

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"You","email":"you@example.com","password":"password123"}'
```

### Projects

All routes below require `Authorization: Bearer <token>`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects?page=1&limit=20` | Projects you **own** or where you have an **assigned** task. Paginated wrapper: `data`, `pagination`. |
| POST | `/projects` | Body: `name` (required), `description` (optional). **201** project. |
| GET | `/projects/{id}` | Project plus `tasks` (up to 500 for this response). |
| PATCH | `/projects/{id}` | Owner only. Partial JSON: `name`, `description`. |
| DELETE | `/projects/{id}` | Owner only. **204** |
| GET | `/projects/{id}/stats` | `by_status` counts and `by_assignee` list. |

```bash
curl -s http://localhost:8080/projects -H "Authorization: Bearer $TOKEN"
```

### Tasks

| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects/{id}/tasks?status=todo&assignee=<uuid>&page=1&limit=20` | Filters optional. Paginated. |
| POST | `/projects/{id}/tasks` | **Owner only.** Body: `title` (required), optional `description`, `status`, `priority`, `assignee_id`, `due_date` (`YYYY-MM-DD`). |
| PATCH | `/tasks/{id}` | **Owner or assignee.** Partial updates. `assignee_id` `""` clears assignee. |
| DELETE | `/tasks/{id}` | **Owner or assignee.** **204** |

```bash
curl -s -X POST "http://localhost:8080/projects/$PID/tasks" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"title":"Ship v1","priority":"high"}'
```

### Health

- `GET /health` → `{"status":"ok"}`

## Postman

Import `postman/TaskFlow.postman_collection.json`. Set collection variables `base_url` and run **Login** (or **Register**); tests store `token`, `project_id`, and `task_id` when present.

## Integration tests

With Postgres available, set `TEST_DATABASE_URL` to a `postgres://` URL (same schema as migrations; an empty database is fine). Then:

```bash
cd backend
# Windows (cmd)
set TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/taskflow?sslmode=disable
# macOS/Linux
export TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/taskflow?sslmode=disable
go test ./internal/handlers/ -count=1
```

Tests truncate `users`, `projects`, and `tasks` before running. If `TEST_DATABASE_URL` is unset, tests **skip**.

## What I would do with more time

- Refresh tokens and tighter JWT rotation; optional **refresh** cookie for browsers.
- **Rate limiting** and account lockout on repeated failed logins.
- Richer **RBAC** (project members) instead of owner-or-assignee-only rules.
- **OpenAPI** spec generated from code or comments; contract tests.
- **OpenTelemetry** tracing and metrics (latency, error rates per route).
- More integration coverage (projects CRUD, stats, pagination edge cases) and CI with a service container.

## Repository layout

```
taskflow/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/...
│   ├── migrations/*.sql (+ embed.go)
│   ├── Dockerfile
│   └── go.mod
├── docker-compose.yml
├── .env.example
├── postman/TaskFlow.postman_collection.json
└── README.md
```
