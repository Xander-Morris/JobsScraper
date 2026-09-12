import { useEffect } from 'react'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { ApiError } from '@/src/api/client'
import { useProfileQuery } from '@/src/api/profile'
import { AuthForms } from '@/src/components/auth/AuthForms'
import { ProfileOutletProvider } from '@/src/components/profile/profile-context'
import { Button } from '@/src/components/ui/button'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useAuth } from '@/src/stores/profile-store'

export const Route = createFileRoute('/profile')({ component: ProfileLayout })

const tabs = [
  { to: '/profile/basic', label: 'Basic info' },
  { to: '/profile/education', label: 'Education' },
  { to: '/profile/skills', label: 'Skills' },
  { to: '/profile/work-experience', label: 'Work experience' },
  { to: '/profile/resumes', label: 'Resumes' }
] as const

function ProfileLayout() {
  const { isAuthenticated, isInitializing } = useAuth()

  if (isInitializing) return <Skeleton className="mx-auto mt-10 h-24 w-full max-w-3xl rounded-xl" />
  if (!isAuthenticated) return <AuthForms />

  return <ProfileContent />
}

function ProfileContent() {
  const { token, logout } = useAuth()
  const { data: profile, error, isLoading, isError } = useProfileQuery(token)
  const isInvalidSession = error instanceof ApiError && (error.status === 401 || error.status === 404)

  useEffect(() => {
    if (isInvalidSession) {
      logout('Your session expired or is no longer valid. Please log in again.')
    }
  }, [isInvalidSession, logout])

  if (isInvalidSession) return <p className="mt-10 text-muted-foreground">Signing you out...</p>

  if (isError) {
    return (
      <p role="alert" className="mt-10 text-muted-foreground">
        Unable to load your profile. Please try again.
      </p>
    )
  }

  if (isLoading || !profile) return <p className="mt-10 text-muted-foreground">Loading profile…</p>

  return (
    <div className="mx-auto mt-8 max-w-2xl text-left mb-4">
      <div className="flex items-center justify-between">
        <h2>{profile.email}</h2>
        <Button type="button" variant="ghost" size="sm" onClick={() => logout()}>
          Log out
        </Button>
      </div>

      <nav aria-label="Profile sections" className="mt-4 flex gap-4 border-b border-border text-sm">
        {tabs.map((tab) => (
          <Link
            key={tab.to}
            to={tab.to}
            className="border-b-2 border-transparent pb-2 text-muted-foreground no-underline hover:text-heading [&.active]:border-heading [&.active]:font-semibold [&.active]:text-heading"
          >
            {tab.label}
          </Link>
        ))}
      </nav>

      <div className="mt-6">
        <ProfileOutletProvider profile={profile}>
          <Outlet />
        </ProfileOutletProvider>
      </div>
    </div>
  )
}
