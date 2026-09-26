import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import './index.css'
import { routeTree } from './routeTree.gen'
import { ProfileAuthProvider, useAuth } from '@/src/stores/auth-store'

// Without a staleTime, every remount and window refocus refetches from
// scratch even when nothing changed - e.g. tabbing back to the jobs list.
// Mutations invalidate their own queries explicitly, so writes still show up
// immediately regardless of this.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000
    }
  }
})

const router = createRouter({
  routeTree,
  context: { queryClient, auth: undefined! },
  // Loaders prefetch on hover; staleTime 0 defers caching to React Query.
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function App() {
  const auth = useAuth()
  return <RouterProvider router={router} context={{ auth }} />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ProfileAuthProvider>
        <App />
      </ProfileAuthProvider>
      <ReactQueryDevtools />
    </QueryClientProvider>
  </StrictMode>
)
