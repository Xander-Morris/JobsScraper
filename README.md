# Jobs Scraper

**Live demo:** [jobs-scraper-vvcj.vercel.app](https://jobs-scraper-vvcj.vercel.app/)

A job board that crawls remote job listings from a handful of public sources (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely), indexes them into Postgres, and lets you build a profile with a resume so it can tell you which listings are actually a good fit. There's also a daily email digest for anyone who wants matching jobs sent to their inbox instead of checking the site.

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
3. Copy `backend/.env.example` to `backend/.env`. Beyond `DATABASE_CONNECTION`, `SECRET_KEY`, `OPENROUTER_API_KEY`, and `JINA_API_KEY`, a public deployment needs `ALLOWED_ORIGIN` (the frontend's origin), `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none` if the frontend is on a different domain (e.g. Vercel), and if digest emails are on, then `PUBLIC_BACKEND_URL` plus a `RESEND_FROM_ADDRESS` on a domain verified in Resend
4. Bring it up:

```bash
docker compose -f docker-compose.prod.yml up --build -d
```

Migrations apply automatically at boot. The database must be Postgres 14+ with the `vector` extension available (Supabase includes it), and should be a different database than `TEST_DATABASE_CONNECTION`. The test suite drops tables.

### Fully on Vercel (free)

Both halves deploy as separate Vercel projects. Neither needs a domain of your own. Each gets a free `*.vercel.app` URL.

**Backend** (project root: `backend/`, entrypoints under `backend/api/`, config in `backend/vercel.json`):

1. Import the repo, set the project's root directory to `backend/`
2. Set env vars: `DATABASE_CONNECTION`, `SECRET_KEY`, `OPENROUTER_API_KEY`, `JINA_API_KEY`, `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none` (frontend and backend are on different Vercel domains), `CRON_SECRET` (any random string; Vercel sends it back as a header to authenticate the two cron endpoints below), and `ALLOWED_ORIGIN` (set once you have the frontend's URL from the next step)
3. Deploy. Every `/api/*` request routes through the single `api/index.go` function (see the `rewrites` entry in `vercel.json`), running the same handler chain as the self-hosted binary
4. `vercel.json` already wires up two Vercel Cron Jobs: a daily job re-scrape (`api/cron/scrape/index.go`) and a daily digest send (`api/cron/digest/index.go`), replacing the self-hosted binary's in-process ticker loops (which have nowhere to live in a serverless deployment). Vercel's free (Hobby) plan caps cron at once/day, so the job board refreshes daily instead of every 10 minutes like the self-hosted version. Vercel Pro allows more frequent schedules if you need that back

**Frontend** (project root: `frontend/`):

1. Import the repo, set the project's root directory to `frontend/`
2. Framework preset: Vite (build command `tsc -b && vite build`, output `dist`, both auto-detected)
3. Set the env var `VITE_API_URL` to the backend project's Vercel URL (baked in at build time)
4. Deploy, then go back and set `ALLOWED_ORIGIN` on the backend project to this URL and redeploy it

## How it fits together

The backend runs two background loops alongside the HTTP server: one that re-scrapes all the job sources every 10 minutes and writes new/updated listings to Postgres, and one that sends the daily digest email to anyone who's opted in (on Vercel, these run as Cron Jobs instead; see Deploying, above). Resume text extraction (turning an uploaded PDF into structured skills/experience) and cover-letter generation go through OpenRouter; resume/job embeddings go through Jina AI.

The frontend talks to the backend over a plain REST API (see `backend/server/routes.go` for the full list of endpoints) using TanStack Query for data fetching and TanStack Router for routing.
