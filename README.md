# Jobs Scraper

**Live demo:** [https://www.jobsscraper.com/](https://www.jobsscraper.com/)

A job board that crawls listings from public job feeds (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely) and from company job boards hosted on Greenhouse, Lever, and Ashby, indexes them into Postgres, and lets you build a profile with a resume so it can tell you which listings are actually a good fit. There's also a daily email digest for anyone who wants matching jobs sent to their inbox instead of checking the site.

## What's in here

- `backend/`: Go API server, background scraper, resume parsing (via OpenRouter + Jina AI), and the digest email job
- `frontend/`: React + Vite single page app for browsing jobs and managing your profile
- `docker-compose.yml` runs the whole stack locally: backend + frontend

Each folder has its own README with setup details specific to that half of the app.

## Quick start

You'll need a Postgres database (Supabase works out of the box, or point it at any Postgres 14+ instance), a free [OpenRouter API key](https://openrouter.ai/keys) and [Jina AI API key](https://jina.ai/api-dashboard), and Docker if you want to use the compose file.

1. Copy `backend/.env.example` to `backend/.env` and fill in `DATABASE_CONNECTION`, `SECRET_KEY`, `OPENROUTER_API_KEY`, and `JINA_API_KEY` at minimum
2. Run everything with:

```bash
docker compose up --build
```

The frontend comes up on `localhost:80`, the backend on `localhost:8090`.

If you'd rather run things without Docker, see the backend and frontend READMEs for running each piece directly with `go run` and `npm run dev`.

## Deploying

Two ways to run this in production: self-hosted behind Docker + Caddy, or fully on Vercel for free.

### Self-hosted (Docker)

`docker-compose.prod.yml` runs the backend behind Caddy, which terminates TLS on `BACKEND_DOMAIN` and is the only container publishing ports. The frontend is a static Vite build and isn't part of this compose file. Deploy it separately (e.g. to Vercel, see below).

Order matters since Caddy requests a certificate on startup, and a failed ACME challenge is rate limited by Let's Encrypt:

1. Point a DNS A record for your backend hostname at the host, and wait for it to resolve
2. Copy `.env.example` to `.env` and set `BACKEND_DOMAIN` and `ACME_EMAIL`
3. Copy `backend/.env.example` to `backend/.env`. Beyond `DATABASE_CONNECTION`, `SECRET_KEY`, `OPENROUTER_API_KEY`, and `JINA_API_KEY`, a public deployment needs `ALLOWED_ORIGIN` (the frontend's origin), `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none` if the frontend is on a different domain (e.g. Vercel), and for digest and password reset emails, `RESEND_API_KEY` plus a `RESEND_FROM_ADDRESS` on a domain verified in Resend (and `PUBLIC_BACKEND_URL` for digest unsubscribe links)
4. Bring it up:

```bash
docker compose -f docker-compose.prod.yml up --build -d
```

The self-hosted binary applies migrations at boot. On Vercel the function doesn't (it would slow every cold start); the `migrate` job in `.github/workflows/ci.yml` applies them on push to `main`. The database must be Postgres 14+ with the `vector` extension available (Supabase includes it), and should be a different database than `TEST_DATABASE_CONNECTION`. The test suite drops tables.

### Fully on Vercel (free)

Both halves deploy as separate Vercel projects. Neither needs a domain of your own. Each gets a free `*.vercel.app` URL.

**Backend** (project root: `backend/`, entrypoints under `backend/api/`, config in `backend/vercel.json`):

1. Import the repo, set the project's root directory to `backend/`
2. Set env vars: `DATABASE_CONNECTION`, `SECRET_KEY`, `OPENROUTER_API_KEY`, `JINA_API_KEY`, `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none` (frontend and backend are on different Vercel domains), `RESEND_API_KEY` and `RESEND_FROM_ADDRESS` (password reset emails; without them nobody can recover a forgotten password), `REDIS_URL` (a free [Upstash](https://upstash.com) Redis database; without it, login and LLM rate limits live in each function instance's memory and are easy to get around), and `ALLOWED_ORIGIN` (set once you have the frontend's URL from the next step)
3. Deploy. Every `/api/*` request routes through the single `api/index.go` function (see the `rewrites` entry in `vercel.json`), running the same handler chain as the self-hosted binary
4. Scraping and digest emails don't run on Vercel, since its free plan caps functions at 60 seconds and cron at once a day. Instead, `.github/workflows/cron.yml` runs them on GitHub Actions (free for public repos): a scrape every hour and a digest daily at 04:15 UTC. In the GitHub repo's Settings → Secrets and variables → Actions, add the secrets `DATABASE_CONNECTION` (use Supabase's pooler URL, since Actions runners have no IPv6), `SECRET_KEY` (the same value as the Vercel project, or digest unsubscribe links break), `JINA_API_KEY`, and `RESEND_API_KEY`, plus the variables `RESEND_FROM_ADDRESS` and `PUBLIC_BACKEND_URL`. Trigger a first run by hand from the Actions tab with "Run workflow"
5. Migrations: the `migrate` job in `.github/workflows/ci.yml` uses the same `DATABASE_CONNECTION` secret to apply pending migrations on every push to `main`, after tests pass. Use Supabase's session pooler URL (port 5432), not the transaction pooler (6543), since golang-migrate holds a session advisory lock. For a brand-new database, run `go run ./cmd/migrate` locally once before the first deploy

**Frontend** (project root: `frontend/`):

1. Import the repo, set the project's root directory to `frontend/`
2. Framework preset: Vite (build command `tsc -b && vite build`, output `dist`, both auto-detected)
3. Set the env vars `VITE_API_URL` to the backend project's Vercel URL and `VITE_MAPBOX_TOKEN` to a Mapbox public token (address autofill), both baked in at build time
4. Deploy, then go back and set `ALLOWED_ORIGIN` on the backend project to this URL and redeploy it

## How it fits together

The backend runs two background loops alongside the HTTP server: one that re-scrapes all the job sources every 10 minutes, writes new/updated listings to Postgres, and deletes postings more than 60 days old (except ones someone marked applied or tailored a resume for), and one that sends the daily digest email to anyone who's opted in (on Vercel, these run on a GitHub Actions schedule instead; see Deploying, above). Resume text extraction (turning an uploaded PDF into structured skills/experience) and cover-letter generation go through OpenRouter; resume/job embeddings go through Jina AI.

The frontend talks to the backend over a plain REST API (see `backend/server/routes.go` for the full list of endpoints) using TanStack Query for data fetching and TanStack Router for routing.
