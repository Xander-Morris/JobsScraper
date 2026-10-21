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
npm test                # vitest, run once
npm run test:watch      # vitest in watch mode
npm run coverage        # vitest with a v8 coverage report in coverage/, fails below 100%
```

## Tests

Vitest with jsdom, React Testing Library and MSW. All tests live under `src/test/`, mirroring `src/` (e.g. `src/components/jobs/job-card.tsx` → `src/test/components/jobs/job-card.test.tsx`), and import code via `@/src/...`. Shared setup sits at the root of `src/test/`:

- `setup.ts` starts MSW (any request without a handler fails the test), stubs browser APIs jsdom lacks, and resets storage and auth between tests.
- `server.ts` holds the MSW server and default handlers; override per test with `server.use(...)`.
- `fixtures.ts` builds API-shaped objects (`makeJob`, `makeProfile`, ...).
- `auth-mock.ts` stands in for the auth store: `vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))`, then `signIn()`.
- `render.tsx` has `renderWithProviders` (query client plus a memory router) and `renderHookWithClient`.

`src/test/routes/routes.test.tsx` mounts the real route tree for page-level flows. `VITE_API_URL` is pinned in `vitest.config.ts`, so a local `.env` can't send test traffic to a real backend.

Coverage must stay at 100% statements, branches, functions and lines (thresholds in `vitest.config.ts`; `src/components/ui/`, `src/main.tsx` and the generated route tree are excluded). Under Vitest the router plugin skips its HMR wrappers, which would otherwise show up as uncovered branches in every route file. `src/test/gate.ts` holds an MSW handler open so tests can check pending states.

CI runs `coverage`, `lint` and `build` on every push and PR, and fails if any `*.test.*` file sits outside `src/test/`.

On pushes and same-repo PRs it first runs `lint:fix` and `format`, commits any changes back to the branch as `style: auto-format [skip ci]`, then fails if lint errors remain. Fork PRs can't be pushed to, so they get `format:check` and `lint` instead.

On Windows with `core.autocrlf=true`, `format:check` flags every file locally because of CRLF line endings. That's harmless, git stores LF and CI checks LF.

## Env vars

- `VITE_API_URL`: backend URL
- `VITE_MAPBOX_TOKEN`: Mapbox public (`pk.`) token for address autofill. Without it the address field still works, just no suggestions. It ships in the bundle, so restrict it to your site's URLs in the Mapbox dashboard

Vite inlines these into the JS bundle at build time, they're not something you can change at runtime after the app is built. That matters for Docker specifically, see below.

## Docker

```bash
docker build --build-arg VITE_API_URL=https://api.yourdomain.com --build-arg VITE_MAPBOX_TOKEN=pk.... -t crawlerandindexer-frontend .
```

Because these get baked into the bundle during `npm run build`, you have to pass them as build args (with `VITE_API_URL` pointing at wherever the backend will actually be reachable), not as runtime environment variables on the container. The image itself is just nginx serving the static build, `nginx.conf` handles the SPA fallback routing so refreshing on a deep link like `/jobs/123` doesn't 404.
