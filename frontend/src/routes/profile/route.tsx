import { useEffect } from 'react'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { ApiError } from '@/src/api/client'
import { profileQueryOptions, useProfileQuery } from '@/src/hooks/use-profile'
import { AuthForms } from '@/src/components/auth/AuthForms'
import { ProfileOutletProvider } from '@/src/components/profile/profile-context'
import { Button } from '@/src/components/ui/button'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useAuth } from '@/src/stores/auth-store'

export const Route = createFileRoute('/profile')({
  loader: ({ context: { auth, queryClient } }) => {
    if (auth.isAuthenticated) void queryClient.prefetchQuery(profileQueryOptions)
  },
  component: ProfileLayout
})

const tabs = [
  { to: '/profile/basic', label: 'Basic info' },
  { to: '/profile/education', label: 'Education' },
  { to: '/profile/skills', label: 'Skills' },
  { to: '/profile/work-experience', label: 'Work experience' },
  { to: '/profile/resumes', label: 'Resumes' }
] as const

function ProfileLayout() {
  const { isAuthenticated, isInitializing } = useAuth()

  if (isInitializing) return <Skeleton className="mx-auto mt-10 h-24 w-full max-w-2xl rounded-xl" />
  if (!isAuthenticated) return <AuthForms />

  return <ProfileContent />
}

function ProfileContent() {
  const { logout } = useAuth()
  const { data: profile, error, isLoading, isError } = useProfileQuery()
  const isInvalidSession = error instanceof ApiError && (error.status === 401 || error.status === 404)

  useEffect(() => {
    if (isInvalidSession) {
      logout('Your session expired or is no longer valid. Please log in again.')
    }
  }, [isInvalidSession, logout])

  if (isInvalidSession) return <p className="mt-10 text-center text-muted-foreground">Signing you out…</p>

  if (isError) {
    return (
      <p role="alert" className="mt-10 text-center text-muted-foreground">
        Unable to load your profile. Please try again.
      </p>
    )
  }

  if (isLoading || !profile) return <Skeleton className="mx-auto mt-10 h-24 w-full max-w-2xl rounded-xl" />

  return (
    <div className="mx-auto max-w-2xl px-4 py-10 sm:px-6">
      <div className="flex items-end justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-3xl font-medium tracking-[-0.03em]">Profile</h1>
          <p className="mt-1 truncate text-sm text-muted-foreground">{profile.email}</p>
        </div>
        <Button type="button" variant="ghost" size="sm" className="md:hidden" onClick={() => logout()}>
          Log out
        </Button>
      </div>

      <nav
        aria-label="Profile sections"
        className="mt-8 flex gap-6 overflow-x-auto border-b border-border text-sm [scrollbar-width:none]"
      >
        {tabs.map((tab) => (
          <Link
            key={tab.to}
            to={tab.to}
            className="-mb-px shrink-0 border-b-2 border-transparent pb-3 text-muted-foreground no-underline transition-colors hover:text-heading [&.active]:border-primary [&.active]:text-heading"
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
