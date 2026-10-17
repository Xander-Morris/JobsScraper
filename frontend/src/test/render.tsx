import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider
} from '@tanstack/react-router'
import { render, renderHook, type RenderHookOptions } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false }
    }
  })
}

// renderWithProviders mounts ui at "/" in a memory router, with a fresh query client.
export async function renderWithProviders(
  ui: ReactElement,
  { client = createTestQueryClient(), path = '/' }: { client?: QueryClient; path?: string } = {}
) {
  const rootRoute = createRootRoute({ component: Outlet })
  const pageRoute = createRoute({ getParentRoute: () => rootRoute, path, component: () => ui })
  const catchAll = createRoute({ getParentRoute: () => rootRoute, path: '$', component: () => <div>elsewhere</div> })

  const router = createRouter({
    routeTree: rootRoute.addChildren([pageRoute, catchAll]),
    history: createMemoryHistory({ initialEntries: [path] })
  })

  await router.load()

  const result = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )

  return { ...result, client, router }
}

export function renderHookWithClient<Result, Props>(
  hook: (props: Props) => Result,
  { client = createTestQueryClient(), ...options }: RenderHookOptions<Props> & { client?: QueryClient } = {}
) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )

  return { ...renderHook(hook, { wrapper, ...options }), client }
}
