# backend

Go API server for CrawlerAndIndexer. Handles auth, job search, profile/resume management, the job scraper, and the digest email scheduler. No framework, just the standard library `net/http` mux plus a handful of focused packages.

## Layout

- `server/` — HTTP handlers, routing, auth middleware, rate limiting
- `database/` — Postgres access, migrations, schema
- `scraper/` — runs the job source fetchers on a 10 minute ticker
- `jobs/` — one file per job source (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely)
- `llm/` — resume text extraction, generation, and embeddings via the Gemini API
- `notify/` — Resend email client for the digest
- `utils/` — env loading and a couple of small helpers
- `api/` — Vercel serverless entrypoints (see the repo root README's "Fully on Vercel" section)
- `coldstart/` — one-time startup work (env validation, migrations) shared by the entrypoints under `api/`

## Running locally

You need Go 1.26+ and a Postgres database.

1. Copy `.env.example` to `.env` and fill in `DATABASE_CONNECTION` (a Postgres connection string) and `SECRET_KEY` (any long random string, `openssl rand -base64 48` works fine)
2. `go run .`

The server listens on `:8090` by default (override with `PORT`). Tables are created/migrated automatically on startup, so there's no separate migration step to run by hand.

Resume extraction, cover letter generation, and semantic job matching need `GEMINI_API_KEY` set (a free key from [aistudio.google.com/apikey](https://aistudio.google.com/apikey)). If you're not testing those features you can skip it, they'll just fail (or silently fall back to keyword matching, for search) until it's set.

## Tests

```bash
go test -p 1 ./...
```

`-p 1` matters: the `database` and `server` packages share one test database and each resets its tables at startup, so run in parallel they wipe each other mid-test.

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
| `GEMINI_API_KEY` | recommended | resume extraction/generation and embeddings no-op or fail without it |
| `GEMINI_MODEL` | optional | defaults to `gemini-2.5-flash` |
| `GEMINI_EMBED_MODEL` | optional | defaults to `text-embedding-004`; must stay 768-dim to match the `vector(768)` columns |
| `CRON_SECRET` | Vercel only | authenticates Vercel Cron Jobs hitting `api/cron/*`; unused by the self-hosted binary |

## API

Routes are all registered in `server/routes.go`. Roughly: job search/detail is public, marking a job as applied and everything under `/api/profile` requires a bearer token, login/signup are rate limited separately from everything else.

Auth is a short-lived JWT access token plus a longer-lived refresh token in an httpOnly cookie. There's no session store, refresh tokens are just rows in Postgres that get invalidated on logout.

## Background jobs

Two loops start alongside the HTTP server and run for the life of the process (self-hosted only — on Vercel, `api/cron/scrape.go` and `api/cron/digest.go` run these as scheduled Cron Jobs instead, since there's no long-lived process to hold a ticker):

- **Scraper** — fetches all job sources immediately on boot, then every 10 minutes
- **Digest scheduler** — sends the "jobs matching your profile" email once on boot, then every 24 hours, to anyone with `email_notifications` on

Both recover from panics per-run so one bad fetch or one bad email doesn't take down the whole loop.

## Docker

```bash
docker build -t crawlerandindexer-backend .
```

Multi-stage build, final image is Alpine with just the compiled binary. See the repo root README and `docker-compose.yml` for running it alongside the frontend, or the "Fully on Vercel" section for the serverless path.
