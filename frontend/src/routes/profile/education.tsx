import { createFileRoute } from '@tanstack/react-router'
import { EducationSection } from '@/src/components/profile/education-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/education')({ component: RouteComponent })

function RouteComponent() {
  const { token, profile } = useProfileOutletContext()

  return <EducationSection token={token} education={profile.education ?? []} />
}
