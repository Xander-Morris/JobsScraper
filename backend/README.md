# backend

Go API server for CrawlerAndIndexer. Handles auth, job search, profile/resume management, the job scraper, and the digest email scheduler. No framework, just the standard library `net/http` mux plus a handful of focused packages.

## Layout

- `server/`: HTTP handlers, routing, auth middleware, rate limiting
- `database/`: Postgres access, migrations, schema
- `scraper/`: runs the job source fetchers on a 10 minute ticker
- `jobs/`: one file per job source: public feeds (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely) and company boards on Greenhouse, Lever, and Ashby, whose company lists sit at the top of `greenhouse.go`, `lever.go`, and `ashby.go`
- `llm/`: resume text extraction and generation via OpenRouter, embeddings via Jina AI
- `notify/`: Resend email client for the digest
- `utils/`: env loading and a couple of small helpers
- `api/`: Vercel serverless entrypoints (see the repo root README's "Fully on Vercel" section)
- `cmd/cron/`: runs one scrape or digest pass and exits, scheduled by `.github/workflows/cron.yml`
- `cmd/migrate/`: applies pending migrations and exits, run by the `migrate` job in `.github/workflows/ci.yml` on push to `main`
- `coldstart/`: one-time startup work (env validation) shared by `api/` and `cmd/cron/`

## Running locally

You need Go 1.26+ and a Postgres database.

1. Copy `.env.example` to `.env` and fill in `DATABASE_CONNECTION` (a Postgres connection string) and `SECRET_KEY` (any long random string, `openssl rand -base64 48` works fine)
2. `go run .`

The server listens on `:8090` by default (override with `PORT`). `go run .` applies migrations on startup, so there's no separate step locally. The Vercel function skips them to keep cold starts fast; run `go run ./cmd/migrate` (or let CI's `migrate` job do it) before deploying a schema change there.

Resume extraction and cover letter generation need `OPENROUTER_API_KEY` set (a free key from [openrouter.ai/keys](https://openrouter.ai/keys)); semantic job matching needs `JINA_API_KEY` too (free from [jina.ai/api-dashboard](https://jina.ai/api-dashboard)). If you're not testing those features you can skip them, they'll just fail (or silently fall back to keyword matching, for search) until set.

## Tests

```bash
go test -p 1 ./...
```

`-p 1` matters: the `database` and `server` packages share one test database and each resets its tables at startup, so run in parallel they wipe each other mid-test.

Tests that touch the database use `TEST_DATABASE_CONNECTION` instead of `DATABASE_CONNECTION`. Make sure that's set to a **different** database than your real one, the test suite drops and recreates tables on it. If `TEST_DATABASE_CONNECTION` isn't set, DB-backed tests skip themselves rather than fail.

CI runs `go vet`, `go build`, and `go test` against a throwaway Postgres service container, so you don't need Docker locally just to get a green check. It also runs `gofmt -w` and commits the result back to the branch (fork PRs just fail on unformatted files instead).

## Config

Full list with comments is in `.env.example`. The short version:

| Var | Required | Notes |
|---|---|---|
| `DATABASE_CONNECTION` | yes | Postgres connection string |
| `SECRET_KEY` | yes | JWT signing key |
| `ALLOWED_ORIGIN` | recommended | CORS origin and base URL for password reset links, defaults to `localhost:5173` which is wrong for any real deploy |
| `COOKIE_SECURE` | recommended | set `true` once you're on HTTPS |
| `REDIS_URL` | recommended on Vercel | shares login/signup and LLM rate limits across serverless instances; falls back to in-memory limits if unset or unreachable |
| `RESEND_API_KEY` | optional | digest and password reset emails no-op (log only, no send) if unset |
| `RESEND_FROM_ADDRESS` | optional | defaults to Resend's shared sandbox address |
| `PUBLIC_BACKEND_URL` | optional | needed for unsubscribe links in digest emails to actually work |
| `TEST_DATABASE_CONNECTION` | dev only | separate DB for `go test` |
| `OPENROUTER_API_KEY` | recommended | resume extraction/generation fail without it |
| `OPENROUTER_MODEL` | optional | defaults to `minimax/minimax-m2.7:free`; must be a `:free` model to stay free |
| `JINA_API_KEY` | recommended | embeddings fail without it (search/digest fall back to keyword matching) |
| `JINA_EMBED_MODEL` | optional | defaults to `jina-embeddings-v2-base-en`; must stay 768-dim to match the `vector(768)` columns |

## API

Routes are all registered in `server/routes.go`. Roughly: job search/detail is public, marking a job as applied and everything under `/api/profile` requires a bearer token, login/signup are rate limited separately from everything else.

Auth is a short-lived JWT access token plus a longer-lived refresh token in an httpOnly cookie. There's no session store, refresh tokens are just rows in Postgres that get invalidated on logout. Password reset emails a one-time link (stored hashed, expires in an hour); using it sets the new password and revokes every refresh token for that profile.

## Background jobs

Two loops start alongside the HTTP server and run for the life of the process (self-hosted only; on Vercel there's no long-lived process to hold a ticker, so `.github/workflows/cron.yml` runs the same work through `cmd/cron` on a GitHub Actions schedule instead: a scrape every hour, a digest daily):

- **Scraper**: fetches all job sources immediately on boot, then every 10 minutes
- **Digest scheduler**: sends the "jobs matching your profile" email once on boot, then every 24 hours, to anyone with `email_notifications` on

Both recover from panics per-run so one bad fetch or one bad email doesn't take down the whole loop.

Each scrape cycle also:

- **Expires old jobs**: postings first published more than 60 days ago are skipped, and stored jobs past that age are deleted (by `posted_at`, or `first_seen_at` for sources with no post date). Jobs someone marked applied or tailored a resume for are kept, but drop out of search.
- **Embeds new jobs**: the title plus the first 2,000 characters of the description go to Jina, which keeps token use down on the free tier. The full description is still stored and shown.
- **Tolerates broken boards**: a Greenhouse, Lever, or Ashby company that fails (renamed board, outage) is logged and skipped without failing the rest.

### Adding a job source

Implement `jobs.JobSource` in a new file under `jobs/` with a `toJob` test, register it in `allSources` in `scraper/scraper.go`, and update the source lists in this README and the root README. For another company on Greenhouse, Lever, or Ashby, add its board slug to that source's list instead.

## Docker

```bash
docker build -t crawlerandindexer-backend .
```

Multi-stage build, final image is Alpine with just the compiled binary. See the repo root README and `docker-compose.yml` for running it alongside the frontend, or the "Fully on Vercel" section for the serverless path.
