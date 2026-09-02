# backend

Go API server for CrawlerAndIndexer. Handles auth, job search, profile/resume management, the job scraper, and the digest email scheduler. No framework, just the standard library `net/http` mux plus a handful of focused packages.

## Layout

- `server/` — HTTP handlers, routing, auth middleware, rate limiting
- `database/` — Postgres access, migrations, schema
- `scraper/` — runs the job source fetchers on a 10 minute ticker
- `jobs/` — one file per job source (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely)
- `llm/` — resume text extraction via Ollama
- `notify/` — Resend email client for the digest
- `utils/` — env loading and a couple of small helpers

## Running locally

You need Go 1.26+ and a Postgres database.

1. Copy `.env.example` to `.env` and fill in `DATABASE_CONNECTION` (a Postgres connection string) and `SECRET_KEY` (any long random string, `openssl rand -base64 48` works fine)
2. `go run .`

The server listens on `:8090` by default (override with `PORT`). Tables are created/migrated automatically on startup, so there's no separate migration step to run by hand.

Resume extraction and cover letter generation need a local Ollama server running with `qwen2.5:7b` pulled, and semantic job matching needs `nomic-embed-text` pulled too. If you're not testing those features you can skip setting it up, they'll just fail (or silently fall back to keyword matching, for search) until Ollama is reachable. Easiest way to get it running is `docker compose up ollama ollama-pull` from the repo root.

## Tests

```bash
go test ./...
```

Tests that touch the database use `TEST_DATABASE_CONNECTION` instead of `DATABASE_CONNECTION`. Make sure that's set to a **different** database than your real one, the test suite drops and recreates tables on it. If `TEST_DATABASE_CONNECTION` isn't set, DB-backed tests skip themselves rather than fail.

CI runs `go vet`, `go build`, and `go test` against a throwaway Postgres service container, so you don't need Docker locally just to get a green check.

## Config

Full list with comments is in `.env.example`. The short version:

| Var | Required | Notes |
|---|---|---|
| `DATABASE_CONNECTION` | yes | Postgres connection string |
| `SECRET_KEY` | yes | JWT signing key |
| `ALLOWED_ORIGIN` | recommended | CORS origin, defaults to `localhost:5173` which is wrong for any real deploy |
| `COOKIE_SECURE` | recommended | set `true` once you're on HTTPS |
| `RESEND_API_KEY` | optional | digest emails no-op (log only, no send) if unset |
| `RESEND_FROM_ADDRESS` | optional | defaults to Resend's shared sandbox address |
| `PUBLIC_BACKEND_URL` | optional | needed for unsubscribe links in digest emails to actually work |
| `TEST_DATABASE_CONNECTION` | dev only | separate DB for `go test` |
| `OLLAMA_BASE_URL` / `OLLAMA_MODEL` | optional | defaults point at `localhost:11434` and `qwen2.5:7b` |
| `OLLAMA_EMBED_MODEL` | optional | embedding model for semantic resume/job matching, defaults to `nomic-embed-text` |

## API

Routes are all registered in `server/routes.go`. Roughly: job search/detail is public, marking a job as applied and everything under `/api/profile` requires a bearer token, login/signup are rate limited separately from everything else.

Auth is a short-lived JWT access token plus a longer-lived refresh token in an httpOnly cookie. There's no session store, refresh tokens are just rows in Postgres that get invalidated on logout.

## Background jobs

Two loops start alongside the HTTP server and run for the life of the process:

- **Scraper** — fetches all job sources immediately on boot, then every 10 minutes
- **Digest scheduler** — sends the "jobs matching your profile" email once on boot, then every 24 hours, to anyone with `email_notifications` on

Both recover from panics per-run so one bad fetch or one bad email doesn't take down the whole loop.

## Docker

```bash
docker build -t crawlerandindexer-backend .
```

Multi-stage build, final image is Alpine with just the compiled binary. See the repo root README and `docker-compose.yml` for running it alongside the frontend and Ollama.
