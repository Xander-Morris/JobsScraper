import { createFileRoute } from '@tanstack/react-router'
import { AuthForms } from '@/src/components/auth/AuthForms'
import ProfileView from '@/src/components/profile/profile-view'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useAuth } from '@/src/stores/profile-store'

export const Route = createFileRoute('/profile/')({ component: RouteComponent })

function RouteComponent() {
  const { token, isAuthenticated, isInitializing } = useAuth()

  if (isInitializing) return <Skeleton className="mx-auto mt-10 h-24 w-full max-w-3xl rounded-xl" />

  return isAuthenticated ? <ProfileView token={token!} /> : <AuthForms />
}
