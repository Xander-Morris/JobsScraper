import { createFileRoute } from '@tanstack/react-router'
import { ResumesSection } from '@/src/components/profile/resumes-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/resumes')({ component: RouteComponent })

function RouteComponent() {
  const { token, profile } = useProfileOutletContext()

  return <ResumesSection token={token} resumes={profile.resumes ?? []} profile={profile} />
}
