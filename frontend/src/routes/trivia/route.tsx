import { createFileRoute, Outlet } from '@tanstack/react-router'
import { AuthForms } from '@/src/components/auth/AuthForms'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useAuth } from '@/src/stores/auth-store'

export const Route = createFileRoute('/trivia')({ component: TriviaLayout })

function TriviaLayout() {
  const { isAuthenticated, isInitializing } = useAuth()

  if (isInitializing) return <Skeleton className="mx-auto mt-10 h-24 w-full max-w-2xl rounded-xl" />
  if (!isAuthenticated) return <AuthForms />

  return (
    <div className="mx-auto max-w-2xl px-4 py-10 sm:px-6">
      <Outlet />
    </div>
  )
}
