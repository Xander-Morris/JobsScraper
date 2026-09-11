import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import './index.css'
import { routeTree } from './routeTree.gen'
import { ProfileAuthProvider } from './stores/profile-store'
import { initTheme } from './lib/theme'

initTheme()

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

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ProfileAuthProvider>
        <RouterProvider router={router} />
      </ProfileAuthProvider>
      <ReactQueryDevtools />
    </QueryClientProvider>
  </StrictMode>
)
