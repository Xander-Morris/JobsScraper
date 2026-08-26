import { createFileRoute } from '@tanstack/react-router'
import { AuthForms } from '@/src/components/auth/AuthForms'
import ProfileView from '@/src/components/profile/profile-view'
import { useAuth } from '@/src/stores/profile-store'

export const Route = createFileRoute('/profile/')({ component: RouteComponent })

function RouteComponent() {
  const { token, isAuthenticated } = useAuth()
  return isAuthenticated ? <ProfileView token={token!} /> : <AuthForms />
}
