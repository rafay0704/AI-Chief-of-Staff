# 🛠️ Local Setup

## Prerequisites

| Tool | Version (tested) | Notes |
|---|---|---|
| Go | 1.26+ | `go version` |
| Docker + Compose | v2/v5 | for Postgres + Redis |
| Node | 24+ | frontend (Batch 5) |
| pnpm | 10+ | frontend package manager |

Optional dev tooling (installed via `cd backend && make tools`):
`air` (live reload), `golangci-lint` (lint). `sqlc` is wired as a Go tool dependency, so
`make sqlc` works without a global install.

## 1. Environment

```bash
cp .env.example .env
```

Edit `.env`. The defaults match `docker-compose.yml`, so locally you usually only set:
- `JWT_SECRET` — any long random string for dev.
- `ANTHROPIC_API_KEY` — only needed from Batch 3 onward.

> `.env` is gitignored. Never commit real secrets. `.env.example` is the committed template.

## 2. Start infrastructure

```bash
make up        # Postgres :5432, Redis :6379, Adminer :8081
make ps        # check health
```

Adminer (DB browser) → http://localhost:8081 — System: PostgreSQL, Server: `postgres`,
User: `acos`, Password: `acos_dev_pw`, Database: `acos`.

## 3. Backend

```bash
cd backend
make tidy         # download Go modules
make migrate-up   # apply DB migrations
make sqlc         # regenerate typed repository code (commit the result)
make run          # API on :8080
```

Verify:

```bash
curl localhost:8080/healthz
curl localhost:8080/readyz
```

## 4. Tests

```bash
cd backend
make test         # unit + integration (integration uses testcontainers → needs Docker running)
make test-unit    # fast unit tests only (-short)
make lint         # golangci-lint
```

## 5. Frontend

Needs the backend (`make run`) and worker (`make worker`) running.

```bash
cd frontend
pnpm install
pnpm dev          # http://localhost:3000
```

The dev server proxies `/api/*` to the backend (`API_PROXY_TARGET`, default `http://localhost:8080`),
so there's no CORS config. Register at http://localhost:3000, add tasks, then **Generate plan**.
Build for production with `pnpm build` (Turbopack; type-checks + lints).

## Common tasks

| Action | Command |
|---|---|
| New migration | create `migrations/000N_name.up.sql` + `.down.sql`, then `make migrate-up` |
| Regenerate DB code | `make sqlc` |
| Roll back one migration | `make migrate-down` |
| Reset DB completely | `make down && docker volume rm $(docker volume ls -q | grep pgdata) && make up && make migrate-up` |

## Troubleshooting

- **`/readyz` fails** → is `make up` healthy? Check `DATABASE_URL` / `REDIS_ADDR` in `.env`.
- **migrate "no change"** → schema already current; fine.
- **testcontainers errors** → Docker daemon must be running and reachable.
