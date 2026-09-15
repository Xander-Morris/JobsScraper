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

Dev server runs on `localhost:5173`. `.env` needs `VITE_API_URL` pointing at your backend (defaults to `http://localhost:8090`, which matches the backend's default port) and `VITE_MAPBOX_TOKEN` for address autocomplete on the profile page.

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

- `VITE_API_URL`: backend URL
- `VITE_MAPBOX_TOKEN`: Mapbox public (`pk.`) token for address autofill. Without it the address field still works, just no suggestions. It ships in the bundle, so restrict it to your site's URLs in the Mapbox dashboard

Vite inlines these into the JS bundle at build time, they're not something you can change at runtime after the app is built. That matters for Docker specifically, see below.

## Docker

```bash
docker build --build-arg VITE_API_URL=https://api.yourdomain.com --build-arg VITE_MAPBOX_TOKEN=pk.... -t crawlerandindexer-frontend .
```

Because these get baked into the bundle during `npm run build`, you have to pass them as build args (with `VITE_API_URL` pointing at wherever the backend will actually be reachable), not as runtime environment variables on the container. The image itself is just nginx serving the static build, `nginx.conf` handles the SPA fallback routing so refreshing on a deep link like `/jobs/123` doesn't 404.
