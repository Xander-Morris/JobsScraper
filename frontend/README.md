# frontend

React + Vite single page app for CrawlerAndIndexer. Browse and filter jobs, manage your profile, upload a resume, and see which listings are a good match.

## Stack

- React 19 + TypeScript
- Vite for the dev server and build
- TanStack Router for routing (file-based, see `src/routes/`)
- TanStack Query for all server data fetching, no manual `fetch` + `useState` in components
- Tailwind v4 + shadcn-style components in `src/components/ui/`
- Zod for validating API responses in `src/api/schemas.ts`

## Running locally

Needs Node 22+.

```bash
npm install
cp .env.example .env
npm run dev
```

Dev server runs on `localhost:5173`. `.env` just needs `VITE_API_URL` pointing at your backend, defaults to `http://localhost:8090` which matches the backend's default port.

Make sure the backend is actually running too, this app doesn't do anything useful against a dead API.

## Scripts

```bash
npm run dev           # dev server with HMR
npm run build          # type check + production build to dist/
npm run lint            # oxlint
npm run lint:fix       # oxlint --fix
npm run format          # prettier --write
npm run format:check   # prettier --check
npm run preview         # serve the production build locally
```

CI runs `lint` and `build` on every push and PR. There's no test suite wired up yet, if you add one, hook it into `.github/workflows/ci.yml` alongside those.

## Env vars

Just one: `VITE_API_URL`. Vite inlines this into the JS bundle at build time, it's not something you can change at runtime after the app is built. That matters for Docker specifically, see below.

## Docker

```bash
docker build --build-arg VITE_API_URL=https://api.yourdomain.com -t crawlerandindexer-frontend .
```

Because `VITE_API_URL` gets baked into the bundle during `npm run build`, you have to pass it as a build arg pointing at wherever the backend will actually be reachable, not as a runtime environment variable on the container. The image itself is just nginx serving the static build, `nginx.conf` handles the SPA fallback routing so refreshing on a deep link like `/jobs/123` doesn't 404.
