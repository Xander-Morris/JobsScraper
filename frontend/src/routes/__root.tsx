import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { Header } from '../components/Header'
import { ProgressBar } from '../components/progress-bar'
import type { ProfileAuthContextValue } from '../stores/auth-store'

export interface RouterContext {
  queryClient: QueryClient
  auth: ProfileAuthContextValue
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent
})

function RootComponent() {
  return (
    <>
      <ProgressBar />
      <Header />
      <main className="flex-1">
        <Outlet />
      </main>
      <TanStackRouterDevtools />
    </>
  )
}
