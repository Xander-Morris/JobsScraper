# CrawlerAndIndexer

**Live demo:** _not deployed yet_

A job board that crawls remote job listings from a handful of public sources (RemoteOK, Remotive, Arbeitnow, Jobicy, Himalayas, WeWorkRemotely), indexes them into Postgres, and lets you build a profile with a resume so it can tell you which listings are actually a good fit. There's also a daily email digest for anyone who wants matching jobs sent to their inbox instead of checking the site.

## What's in here

- `backend/` — Go API server, background scraper, resume parsing (via a local Ollama model), and the digest email job
- `frontend/` — React + Vite single page app for browsing jobs and managing your profile
- `docker-compose.yml` — runs the whole stack locally: backend, frontend, and an Ollama container for resume extraction

Each folder has its own README with setup details specific to that half of the app.

## Quick start

You'll need a Postgres database (Supabase works out of the box, or point it at any Postgres 14+ instance) and Docker if you want to use the compose file.

1. Copy `backend/.env.example` to `backend/.env` and fill in `DATABASE_CONNECTION` and `SECRET_KEY` at minimum
2. Run everything with:

```bash
docker compose up --build
```

The frontend comes up on `localhost:80`, the backend on `localhost:8090`. First boot pulls the `qwen2.5:7b` Ollama model automatically, which takes a few minutes and needs roughly 8GB of RAM to run comfortably.

If you'd rather run things without Docker, see the backend and frontend READMEs for running each piece directly with `go run` and `npm run dev`.

## Deploying

`docker-compose.prod.yml` runs the same stack behind Caddy, which terminates TLS and is the only container publishing ports. The host needs ~8GB of RAM, since Ollama keeps `qwen2.5:7b` resident for resume extraction.

Order matters — Caddy requests certificates on startup, and a failed ACME challenge is rate limited by Let's Encrypt:

1. Point DNS A records for both your frontend and backend hostnames at the host, and wait for them to resolve
2. Copy `.env.example` to `.env` and set `FRONTEND_DOMAIN`, `BACKEND_DOMAIN`, and `ACME_EMAIL`
3. Copy `backend/.env.example` to `backend/.env`. Beyond `DATABASE_CONNECTION` and `SECRET_KEY`, a public deployment needs `ALLOWED_ORIGIN`, `COOKIE_SECURE=true`, and — if digest emails are on — `PUBLIC_BACKEND_URL` plus a `RESEND_FROM_ADDRESS` on a domain verified in Resend
4. Bring it up:

```bash
docker compose -f docker-compose.prod.yml up --build -d
```

Migrations apply automatically at boot. The database must be Postgres 14+ with the `vector` extension available (Supabase includes it), and should be a different database than `TEST_DATABASE_CONNECTION` — the test suite drops tables.

## How it fits together

The backend runs two background loops alongside the HTTP server: one that re-scrapes all the job sources every 10 minutes and writes new/updated listings to Postgres, and one that sends the daily digest email to anyone who's opted in. Resume text extraction (turning an uploaded PDF into structured skills/experience) is done by a local LLM through Ollama rather than an external API, so there's no per-resume API cost and nothing leaves your infrastructure.

The frontend talks to the backend over a plain REST API (see `backend/server/routes.go` for the full list of endpoints) using TanStack Query for data fetching and TanStack Router for routing.
