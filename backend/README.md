# backend

Go API server for CrawlerAndIndexer. Handles auth, job search, profile/resume management, trivia quizzes, the job scraper, and the digest email scheduler. No framework, just the standard library `net/http` mux plus a handful of focused packages.

## Layout

- `server/`: HTTP handlers, routing, auth middleware, rate limiting
- `database/`: Postgres access, migrations, schema
- `scraper/`: runs the job source fetchers on a 10 minute ticker
- `jobs/`: one file per job source: public feeds (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely) and company boards on Greenhouse, Lever, and Ashby, whose company lists sit at the top of `greenhouse.go`, `lever.go`, and `ashby.go`
- `crawler/`: a web crawler for career sites with no API (NVIDIA, Salesforce, Adobe, Intel, Mastercard, Cisco, Red Hat, Target, HP, all on Workday), seeded from `seeds.go`. See "Crawler" below
- `llm/`: resume text extraction, generation, and trivia quizzes via OpenRouter, embeddings via Jina AI
- `notify/`: Resend email client for the digest
- `utils/`: env loading and a couple of small helpers
- `api/`: Vercel serverless entrypoints (see the repo root README's "Fully on Vercel" section)
- `cmd/cron/`: runs one scrape or digest pass and exits, scheduled by `.github/workflows/cron.yml`
- `cmd/migrate/`: applies pending migrations and exits, run by the `migrate` job in `.github/workflows/ci.yml` on push to `main`
- `coldstart/`: one-time startup work (env validation) shared by `api/` and `cmd/cron/`

## Running locally

You need Go 1.26+ and a Postgres database.

1. Copy `.env.example` to `.env` and fill in `DATABASE_CONNECTION` (a Postgres connection string) and `SECRET_KEY` (any long random string, `openssl rand -base64 48` works fine)
2. `go run ./cmd/server`

The server listens on `:8090` by default (override with `PORT`). `go run ./cmd/server` applies migrations on startup, so there's no separate step locally. The Vercel function skips them to keep cold starts fast; run `go run ./cmd/migrate` (or let CI's `migrate` job do it) before deploying a schema change there.

Resume extraction, cover letter generation, and trivia need `OPENROUTER_API_KEY` set (a free key from [openrouter.ai/keys](https://openrouter.ai/keys)); semantic job matching needs `JINA_API_KEY` too (free from [jina.ai/api-dashboard](https://jina.ai/api-dashboard)). If you're not testing those features you can skip them, they'll just fail (or silently fall back to keyword matching, for search) until set.

## Tests

```bash
go test -p 1 ./...
```

`-p 1` matters: the `database`, `server`, `digest` and `scraper` packages share one test database and each resets its tables at startup, so run in parallel they wipe each other mid-test.

Tests that touch the database use `TEST_DATABASE_CONNECTION` instead of `DATABASE_CONNECTION`. Make sure that's set to a **different** database than your real one, the test suite drops every table on it. If `TEST_DATABASE_CONNECTION` isn't set, DB-backed tests skip themselves rather than fail.

To run the DB tests locally, start the same Postgres image CI uses:

```bash
docker run -d --name crawler-test-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=crawlerandindexer_test -p 55432:5432 pgvector/pgvector:pg16
TEST_DATABASE_CONNECTION=postgresql://postgres:postgres@localhost:55432/crawlerandindexer_test go test -p 1 ./...
```

### Coverage

CI requires 100% statement coverage. Check it locally with:

```bash
go test -p 1 -coverpkg=./... -coverprofile=cover.out ./...
grep -v "main/internal/" cover.out > cover.app.out
go tool cover -func=cover.app.out | grep -v "100.0%"
```

`-coverpkg=./...` counts code a test reaches in another package, so the API tests count toward `database`. `internal/faultdb` is test-only tooling and is left out of the total. It wraps the Postgres driver so a test can fail, or corrupt the rows of, the Nth database call; `faultdb.Sweep` reruns a call failing each of its calls in turn and checks the error comes back. That's how the `if err != nil` branches a healthy database never takes get covered.

The entrypoints (`cmd/server`, `cmd/cron`, `cmd/migrate`, `api`) keep `main` to one line and put the work in a `run` function that returns an exit code. Package-level vars (`exit`, `ensure`, ...) let tests stub out exiting and startup.

No test calls a real outside service. OpenRouter, Jina and Resend are faked with local servers through `OPENROUTER_API_URL`, `JINA_API_URL` and `RESEND_API_URL`, and the signup email check is swapped for an offline one (`database.SetEmailVerifier`), so no DNS lookups either. The `server` tests drive the full HTTP handler (routing, auth, CORS) as well as calling handlers directly.

CI runs `go vet`, `go build`, and `go test` with the coverage check against a throwaway Postgres service container, so you don't need Docker locally just to get a green check. It also runs `gofmt -w` and commits the result back to the branch (fork PRs just fail on unformatted files instead).

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
| `OPENROUTER_MODEL` | optional | defaults to `nvidia/nemotron-3-super-120b-a12b:free`; must be a `:free` model to stay free, and reasoning is sent disabled |
| `JINA_API_KEY` | recommended | embeddings fail without it (search/digest fall back to keyword matching) |
| `JINA_EMBED_MODEL` | optional | defaults to `jina-embeddings-v2-base-en`; must stay 768-dim to match the `vector(768)` columns |

## API

Routes are all registered in `server/routes.go`. Roughly: job search/detail is public, marking a job as applied and everything under `/api/profile` requires a bearer token, login/signup are rate limited separately from everything else.

Auth is a short-lived JWT access token plus a longer-lived refresh token in an httpOnly cookie. There's no session store, refresh tokens are just rows in Postgres that get invalidated on logout. Checking "Remember me" at login gives a 30-day persistent cookie; leaving it unchecked ends the session once the user leaves the site (the frontend revokes it on the next visit if no other tab is still open), with a browser-session cookie and a one-day idle expiry as backstops. Password reset emails a one-time link (stored hashed, expires in an hour); using it sets the new password and revokes every refresh token for that profile.

## Background jobs

Two loops start alongside the HTTP server and run for the life of the process (self-hosted only; on Vercel there's no long-lived process to hold a ticker, so `.github/workflows/cron.yml` runs the same work through `cmd/cron` on a GitHub Actions schedule instead: a scrape every hour, a digest daily):

- **Scraper**: fetches all job sources immediately on boot, then every 10 minutes
- **Digest scheduler**: sends the "jobs matching your profile" email once on boot, then every 24 hours, to anyone with `email_notifications` on

Both recover from panics per-run so one bad fetch or one bad email doesn't take down the whole loop.

Each scrape cycle also:

- **Expires old jobs**: postings first published more than 30 days ago are skipped, and stored jobs past that age are deleted (by `posted_at`, or `first_seen_at` for sources with no post date). Stored jobs are also capped at the newest 5,000 (`MaxJobs`); older ones past the cap are deleted, and each scrape writes at most that many. Jobs someone marked applied or tailored a resume for are kept, but drop out of search. Pruning runs before each write, so it still frees space if Supabase has put the database in read-only mode.
- **Embeds new jobs**: the title plus the first 2,000 characters of the description go to Jina, which keeps token use down on the free tier. The full description is still stored and shown.
- **Normalizes salaries**: structured pay (Himalayas, Jobicy, Ashby compensation, Lever `salaryRange`, JSON-LD `baseSalary`) is converted to yearly USD; hourly, weekly, and monthly rates are annualized and other currencies are dropped. Jobs with no structured pay fall back to reading a range like "$150,000 – $200,000" that follows a word like "salary" or "pay" in the description (`jobs/salary.go`).
- **Tolerates broken boards**: a Greenhouse, Lever, Ashby company, or crawler seed that fails (renamed board, outage) is logged and skipped without failing the rest.

### Crawler

The other sources read JSON or RSS feeds. The crawler instead loads real career site pages and scrapes them. For each seed in `crawler/seeds.go` it:

1. **Reads `robots.txt`**: obeys `Disallow`/`Allow` for the `JobsScraper` agent (or `*`), uses its `Crawl-delay` if it's longer than the default 1 second between requests to a host, and collects its `Sitemap:` lines. If `robots.txt` is unreachable (5xx or a network error), that seed is skipped for this cycle, as RFC 9309 says to.
2. **Discovers pages**: walks the host's sitemaps (sitemap indexes and `.gz` too), newest `lastmod` first, then follows links from the seed page up to two hops away. Only URLs under the seed URL's path are fetched, and a seed's `Include` pattern can narrow that further.
3. **Scrapes each page**: parses the HTML and reads the schema.org `JobPosting` JSON-LD that career sites embed for Google for Jobs (title, description, post date, locations, remote flag, employment type, salary). Postings past their `validThrough` date are dropped. Pages that only render with JavaScript have no JSON-LD in their HTML and yield nothing.

Storage stays bounded so it fits a free database:

- At most 40 pages per seed per scrape, and a whole crawl stops after 6 minutes
- URLs already stored are skipped before fetching (`database.KnownJobURLs`), so each scrape's budget goes to new postings instead of reloading old ones
- Descriptions are cut at 20,000 characters
- Crawled jobs share the same 30-day and 5,000-job limits as every other source

To crawl another site, add a `Seed` to `crawler/seeds.go`. Any site whose job pages carry `JobPosting` JSON-LD in their HTML works; for another company on Workday, use `workday(siteURL, company)` with the career site URL from its `robots.txt`. Set `Company`, since `hiringOrganization` is often a legal entity name like "2100 NVIDIA USA".

### Adding a job source

Implement `jobs.JobSource` in a new file under `jobs/` with a `toJob` test, register it in `allSources` in `scraper/scraper.go`, and update the source lists in this README and the root README. For another company on Greenhouse, Lever, or Ashby, add its board slug to that source's list instead, and for a career site with no API, add a crawler seed (see "Crawler" above).

## Docker

```bash
docker build -t crawlerandindexer-backend .
```

Multi-stage build, final image is Alpine with just the compiled binary. See the repo root README and `docker-compose.yml` for running it alongside the frontend, or the "Fully on Vercel" section for the serverless path.
