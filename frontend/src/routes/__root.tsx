import { createRootRoute, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { Header } from '../components/header'

export const Route = createRootRoute({
  component: RootComponent
})

function RootComponent() {
  return (
    <>
      <Header />
      <main className="flex-1">
        <Outlet />
      </main>
      <TanStackRouterDevtools />
    </>
  )
}
