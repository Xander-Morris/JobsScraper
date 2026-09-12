import { createFileRoute } from '@tanstack/react-router'
import { ResumesSection } from '@/src/components/profile/resumes-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/resumes')({ component: RouteComponent })

function RouteComponent() {
  const { profile } = useProfileOutletContext()

  return <ResumesSection resumes={profile.resumes ?? []} profile={profile} />
}
